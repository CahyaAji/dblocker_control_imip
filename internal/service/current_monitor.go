package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

const failThreshold = 5

// minCurrentDelta is the minimum increase (in amps) the live reading
// must show over the saved snapshot to be considered valid.
const minCurrentDelta = 1.0

// SectorCurrents holds the three current readings for a single sector.
type SectorCurrents struct {
	Ctrl1 float64 `json:"ctrl1"`
	Ctrl2 float64 `json:"ctrl2"`
	GPS   float64 `json:"gps"`
}

// SectorFailCounts tracks consecutive fail counts per signal per sector.
type SectorFailCounts struct {
	Ctrl1 int `json:"ctrl1"`
	Ctrl2 int `json:"ctrl2"`
	GPS   int `json:"gps"`
}

// MonitorStatus is the per-device status exposed to the frontend.
type MonitorStatus struct {
	Errors []string `json:"errors"`
}

// SectorConfig mirrors the relevant fields from DBlockerConfig for comparison.
type SectorConfig struct {
	Ctrl bool
	GPS  bool
}

// deviceState holds everything the monitor needs for one device.
type deviceState struct {
	snapshot   []SectorCurrents   // saved at config-apply time
	failCounts []SectorFailCounts // 6 sectors
	config     []SectorConfig     // which signals are ON
}

// CurrentMonitorService compares live /rpt readings against a snapshot
// taken at config-apply time and tracks consecutive failures centrally.
type CurrentMonitorService struct {
	mu      sync.RWMutex
	devices map[string]*deviceState // keyed by serial_numb
}

func NewCurrentMonitorService() *CurrentMonitorService {
	return &CurrentMonitorService{
		devices: make(map[string]*deviceState),
	}
}

// Snapshot saves the current live readings and the active config for a device.
// Called by the config-update handler right before publishing the new command.
func (m *CurrentMonitorService) Snapshot(serial string, rptPayload string, cfg []SectorConfig) {
	currents := parseRptPayload(rptPayload)
	if currents == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	fc := make([]SectorFailCounts, 6)
	snap := make([]SectorCurrents, len(currents))
	copy(snap, currents)
	cfgCopy := make([]SectorConfig, len(cfg))
	copy(cfgCopy, cfg)

	m.devices[serial] = &deviceState{
		snapshot:   snap,
		failCounts: fc,
		config:     cfgCopy,
	}
}

// HandleRpt processes an incoming /rpt payload for a device.
func (m *CurrentMonitorService) HandleRpt(serial string, payload string) {
	currents := parseRptPayload(payload)
	if currents == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ds, ok := m.devices[serial]
	if !ok || ds.snapshot == nil {
		return
	}

	for s := 0; s < 6 && s < len(ds.snapshot) && s < len(currents); s++ {
		saved := ds.snapshot[s]
		live := currents[s]
		cfg := ds.config[s]

		if cfg.Ctrl {
			if (live.Ctrl1 - saved.Ctrl1) < minCurrentDelta {
				ds.failCounts[s].Ctrl1++
			} else {
				ds.failCounts[s].Ctrl1 = 0
			}
			if (live.Ctrl2 - saved.Ctrl2) < minCurrentDelta {
				ds.failCounts[s].Ctrl2++
			} else {
				ds.failCounts[s].Ctrl2 = 0
			}
		} else {
			ds.failCounts[s].Ctrl1 = 0
			ds.failCounts[s].Ctrl2 = 0
		}

		if cfg.GPS {
			if (live.GPS - saved.GPS) < minCurrentDelta {
				ds.failCounts[s].GPS++
			} else {
				ds.failCounts[s].GPS = 0
			}
		} else {
			ds.failCounts[s].GPS = 0
		}
	}
}

// Status returns the current monitor status for a device.
func (m *CurrentMonitorService) Status(serial string) MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ds, ok := m.devices[serial]
	if !ok || ds.snapshot == nil {
		return MonitorStatus{Errors: []string{}}
	}

	var errors []string
	for s := 0; s < 6; s++ {
		fc := ds.failCounts[s]
		if fc.Ctrl1 >= failThreshold {
			errors = append(errors, fmt.Sprintf("S%d RC1", s+1))
		}
		if fc.Ctrl2 >= failThreshold {
			errors = append(errors, fmt.Sprintf("S%d RC2", s+1))
		}
		if fc.GPS >= failThreshold {
			errors = append(errors, fmt.Sprintf("S%d GPS", s+1))
		}
	}

	if errors == nil {
		errors = []string{}
	}
	return MonitorStatus{Errors: errors}
}

// StatusAll returns the monitor status for all tracked devices.
func (m *CurrentMonitorService) StatusAll() map[string]MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]MonitorStatus, len(m.devices))
	for serial := range m.devices {
		// Inline the status logic to avoid re-locking
		ds := m.devices[serial]
		var errors []string
		for s := 0; s < 6; s++ {
			fc := ds.failCounts[s]
			if fc.Ctrl1 >= failThreshold {
				errors = append(errors, fmt.Sprintf("S%d RC1", s+1))
			}
			if fc.Ctrl2 >= failThreshold {
				errors = append(errors, fmt.Sprintf("S%d RC2", s+1))
			}
			if fc.GPS >= failThreshold {
				errors = append(errors, fmt.Sprintf("S%d GPS", s+1))
			}
		}
		if errors == nil {
			errors = []string{}
		}
		result[serial] = MonitorStatus{Errors: errors}
	}
	return result
}

// --- Parsing helpers ---

func calculateCurrentA(rawADC float64) float64 {
	vcc := 3.3
	voltage := rawADC * (vcc / 1023.0)
	vZero := vcc / 2.0
	sensitivity := 0.0396
	return (voltage - vZero) / sensitivity
}

func parseRptPayload(payload string) []SectorCurrents {
	parts := strings.Split(payload, "|")
	numericPart := parts[0]
	if numericPart == "" {
		return nil
	}

	fields := strings.Split(numericPart, ",")
	if len(fields) < 18 {
		return nil
	}

	values := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil || math.IsNaN(v) {
			return nil
		}
		values = append(values, v)
	}

	if len(values) < 18 {
		return nil
	}

	sectors := make([]SectorCurrents, 6)
	for s := 0; s < 6; s++ {
		sectors[s] = SectorCurrents{
			Ctrl1: calculateCurrentA(values[s*3]),
			Ctrl2: calculateCurrentA(values[s*3+1]),
			GPS:   calculateCurrentA(values[s*3+2]),
		}
	}

	return sectors
}
