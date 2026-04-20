package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultFanOnTemp  = 45.0 // turn fans ON above this temperature (°C)
	defaultFanOffTemp = 35.0 // turn fans OFF below this temperature (°C)
	minTempGap        = 5.0  // minimum difference between ON and OFF thresholds

	defaultTempWarnLimit = 55.0 // show warning above this temperature (°C)
	defaultTempOffLimit  = 65.0 // auto turn-off above this temperature (°C)
	minTempLimitGap      = 5.0  // minimum difference between warn and off limits
)

type fanDeviceState struct {
	fansOn      bool
	temperature float64
	hasTemp     bool
	anySectorOn bool
	autoOffSent bool // prevents repeated auto-off commands
}

// AutoOffCallback is called when temperature exceeds the off limit.
// serial is the device serial number.
type AutoOffCallback func(serial string)

// FanControlService manages automatic fan control per device.
// Rules:
//  1. If any sector is ON → both fans ON.
//  2. If no sector is ON and temperature > fanOnTemp → fans ON.
//  3. If no sector is ON and temperature < fanOffTemp → fans OFF.
//  4. Between fanOffTemp–fanOnTemp with no sector ON → keep previous state (hysteresis).
type FanControlService struct {
	mu            sync.RWMutex
	devices       map[string]*fanDeviceState
	client        mqtt.Client
	fanOnTemp     float64
	fanOffTemp    float64
	tempWarnLimit float64
	tempOffLimit  float64
	autoOffCb     AutoOffCallback
}

func NewFanControlService(client mqtt.Client) *FanControlService {
	return &FanControlService{
		devices:       make(map[string]*fanDeviceState),
		client:        client,
		fanOnTemp:     defaultFanOnTemp,
		fanOffTemp:    defaultFanOffTemp,
		tempWarnLimit: defaultTempWarnLimit,
		tempOffLimit:  defaultTempOffLimit,
	}
}

// SetAutoOffCallback sets the callback invoked when a device exceeds the off temperature limit.
func (f *FanControlService) SetAutoOffCallback(cb AutoOffCallback) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.autoOffCb = cb
}

// InitDevice sets the initial sector-on state for a device (called on startup).
func (f *FanControlService) InitDevice(serial string, cfg []models.DBlockerConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ds := f.getOrCreate(serial)
	ds.anySectorOn = anySectorOn(cfg)
	ds.autoOffSent = false
}

// FanState computes the fan state for a device given the new config.
// Called by the handler before publishing a config command.
// Returns (fanMaster, fanSlave).
func (f *FanControlService) FanState(serial string, cfg []models.DBlockerConfig) (bool, bool) {
	on := anySectorOn(cfg)

	f.mu.Lock()
	ds := f.getOrCreate(serial)
	ds.anySectorOn = on

	if on {
		ds.fansOn = true
		f.mu.Unlock()
		return true, true
	}

	// No sector ON — check temperature
	if ds.hasTemp {
		if ds.temperature > f.fanOnTemp {
			ds.fansOn = true
		} else if ds.temperature < f.fanOffTemp {
			ds.fansOn = false
		}
		// between thresholds: keep current state (hysteresis)
	} else {
		ds.fansOn = false
	}

	result := ds.fansOn
	f.mu.Unlock()
	return result, result
}

// HandleTemperature processes temperature from an /rpt payload.
// If the temperature crosses a threshold while no sectors are ON,
// it publishes a fan-only MQTT command.
func (f *FanControlService) HandleTemperature(serial string, payload string) {
	temp, ok := parseTemperature(payload)
	if !ok {
		return
	}

	f.mu.Lock()
	ds := f.getOrCreate(serial)
	ds.temperature = temp
	ds.hasTemp = true

	// If sectors are ON, fans are already ON — skip fan threshold logic but still check temp limits.
	if ds.anySectorOn {
		f.mu.Unlock()
		f.checkTempLimits(serial, temp)
		return
	}

	prevFansOn := ds.fansOn
	if temp > f.fanOnTemp {
		ds.fansOn = true
	} else if temp < f.fanOffTemp {
		ds.fansOn = false
	}

	newFansOn := ds.fansOn
	f.mu.Unlock()

	if newFansOn != prevFansOn {
		f.sendFanOnlyCommand(serial, newFansOn)
	}

	// Check temperature limits for warning/auto-off
	f.checkTempLimits(serial, temp)
}

// TemperatureAll returns the latest temperature for every tracked device.
func (f *FanControlService) TemperatureAll() map[string]float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]float64, len(f.devices))
	for serial, ds := range f.devices {
		if ds.hasTemp {
			result[serial] = ds.temperature
		}
	}
	return result
}

// GetThresholds returns the current fan ON/OFF temperature thresholds.
func (f *FanControlService) GetThresholds() (onTemp, offTemp float64) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fanOnTemp, f.fanOffTemp
}

// SetThresholds updates the fan ON/OFF temperature thresholds.
// Returns an error if onTemp - offTemp < minTempGap.
func (f *FanControlService) SetThresholds(onTemp, offTemp float64) error {
	if onTemp-offTemp < minTempGap {
		return fmt.Errorf("fan_on_temp must be at least %.0f°C higher than fan_off_temp", minTempGap)
	}
	f.mu.Lock()
	f.fanOnTemp = onTemp
	f.fanOffTemp = offTemp
	f.mu.Unlock()
	log.Printf("fan_control: thresholds updated: ON > %.1f°C, OFF < %.1f°C", onTemp, offTemp)
	return nil
}

// GetTempLimits returns the current warning and auto-off temperature limits.
func (f *FanControlService) GetTempLimits() (warnLimit, offLimit float64) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.tempWarnLimit, f.tempOffLimit
}

// SetTempLimits updates the warning and auto-off temperature limits.
func (f *FanControlService) SetTempLimits(warnLimit, offLimit float64) error {
	if offLimit-warnLimit < minTempLimitGap {
		return fmt.Errorf("auto-off temperature must be at least %.0f°C higher than warning temperature", minTempLimitGap)
	}
	if warnLimit < 0 || offLimit < 0 {
		return fmt.Errorf("temperature limits must be positive")
	}
	f.mu.Lock()
	f.tempWarnLimit = warnLimit
	f.tempOffLimit = offLimit
	// Reset auto-off flags so devices can be re-evaluated
	for _, ds := range f.devices {
		ds.autoOffSent = false
	}
	f.mu.Unlock()
	log.Printf("fan_control: temp limits updated: warn > %.1f°C, auto-off > %.1f°C", warnLimit, offLimit)
	return nil
}

// checkTempLimits triggers auto-off when temperature exceeds the off limit.
func (f *FanControlService) checkTempLimits(serial string, temp float64) {
	f.mu.Lock()
	ds := f.getOrCreate(serial)
	offLimit := f.tempOffLimit
	cb := f.autoOffCb

	if temp > offLimit && !ds.autoOffSent {
		ds.autoOffSent = true
		f.mu.Unlock()
		log.Printf("fan_control: temp %.1f°C > %.1f°C for %s, triggering auto-off", temp, offLimit, serial)
		if cb != nil {
			cb(serial)
		}
		return
	}

	// Reset auto-off flag when temperature drops below warning limit
	if temp < f.tempWarnLimit && ds.autoOffSent {
		ds.autoOffSent = false
	}
	f.mu.Unlock()
}

// IsOverheating returns true if the device temperature exceeds the auto-off limit.
func (f *FanControlService) IsOverheating(serial string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	ds, ok := f.devices[serial]
	if !ok || !ds.hasTemp {
		return false
	}
	return ds.temperature > f.tempOffLimit
}

// sendFanOnlyCommand publishes a config with all sectors OFF and only fan bits set.
func (f *FanControlService) sendFanOnlyCommand(serial string, fansOn bool) {
	allOff := make([]models.DBlockerConfig, 6)
	bitmask, err := DBlockerConfigToBitmask(allOff, fansOn, fansOn)
	if err != nil {
		log.Printf("fan_control: failed to build bitmask for %s: %v", serial, err)
		return
	}

	topic := fmt.Sprintf("dbl/%s/cmd", serial)
	payload := []byte{byte(bitmask >> 8), byte(bitmask)}

	if err := f.client.Publish(topic, 1, true, payload); err != nil {
		log.Printf("fan_control: failed to publish fan command for %s: %v", serial, err)
	} else if fansOn {
		log.Printf("fan_control: temp > %.0f°C for %s, fans ON", f.fanOnTemp, serial)
	} else {
		log.Printf("fan_control: temp < %.0f°C for %s, fans OFF", f.fanOffTemp, serial)
	}
}

func (f *FanControlService) getOrCreate(serial string) *fanDeviceState {
	ds, ok := f.devices[serial]
	if !ok {
		ds = &fanDeviceState{}
		f.devices[serial] = ds
	}
	return ds
}

func anySectorOn(cfg []models.DBlockerConfig) bool {
	for _, c := range cfg {
		if c.SignalGPS || c.SignalCtrl {
			return true
		}
	}
	return false
}

// parseTemperature extracts temperature in Celsius from an /rpt payload.
// Supports both MCU formats:
//   - Original master (20 fields): fields[18],fields[19] = temp×100
//   - MCU 3.x (19 fields): fields[18] = raw ADC (STM32 12-bit, LM35)
func parseTemperature(payload string) (float64, bool) {
	parts := strings.Split(payload, "|")
	numericPart := parts[0]
	if numericPart == "" {
		return 0, false
	}

	fields := strings.Split(numericPart, ",")
	if len(fields) < 19 {
		return 0, false
	}

	if len(fields) >= 20 {
		// Original MCU: 2 temperature values, each = Celsius × 100
		t1, err1 := strconv.ParseFloat(strings.TrimSpace(fields[18]), 64)
		t2, err2 := strconv.ParseFloat(strings.TrimSpace(fields[19]), 64)

		t1Valid := err1 == nil && t1 != -9900
		t2Valid := err2 == nil && t2 != -9900

		if t1Valid && t2Valid {
			return max(t1, t2) / 100.0, true
		} else if t1Valid {
			return t1 / 100.0, true
		} else if t2Valid {
			return t2 / 100.0, true
		}
		return 0, false
	}

	// MCU 3.x: single raw ADC value (STM32 12-bit, 3.3V ref, LM35 10mV/°C)
	raw, err := strconv.ParseFloat(strings.TrimSpace(fields[18]), 64)
	if err != nil {
		return 0, false
	}
	voltage := raw * 3.3 / 4095.0
	tempC := voltage * 100.0
	return tempC, true
}
