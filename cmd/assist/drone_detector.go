package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var startFlag = []byte{0xEE, 0xEE, 0xEE, 0xEE}

// holdSeconds is the minimum time (seconds) a dblocker stays ON after the last detection.
// It is refreshed from the backend API by startHoldSettingSync.
var (
	holdSeconds   int = 30
	holdSecondsMu sync.RWMutex
)

// holdTimers tracks per-serial auto-off timers.
var (
	holdTimersMu sync.Mutex
	holdTimers   = map[string]*time.Timer{}
)

// autoBlockerEnabled and autoCameraEnabled control whether detections trigger
// dblocker activation and camera PTZ movement respectively.
// When disabled, detections still appear in the live console.
var (
	autoBlockerEnabled bool = true
	autoCameraEnabled  bool = true
	autoFlagsMu        sync.RWMutex
)

// Detection quality filters. 0 = disabled for confidence; 0 = disabled for signal (signal is negative dB).
// Refreshed from backend settings via startHoldSettingSync.
var (
	detectionMinConfidence     uint8   = 0
	detectionMinSignalStrength float64 = 0
	detectionFilterMu          sync.RWMutex
)

// whitelistUniqueIDs and whitelistTargetNames cache the drone whitelist from the backend.
// Drones matching either set are logged but will NOT trigger dblocker activation.
// Refreshed on the same 60-second polling cycle as hold settings.
var (
	whitelistUniqueIDs   = map[string]bool{}
	whitelistTargetNames = map[string]bool{}
	whitelistMu          sync.RWMutex
)

// pendingRestores tracks dblockers that need a default-config restore but were
// offline when the hold timer fired. The retry loop replays them every 5s.
var (
	pendingRestoresMu sync.Mutex
	pendingRestores   = map[string]string{} // serial → label
)

// mqttDetectPublisher is set by main() after MQTT connects.
// If nil, MQTT publishing is skipped.
var mqttDetectPublisher mqttPublisher

// mqttPublisher is a minimal interface so we don't import the full MQTT package here.
type mqttPublisher interface {
	Publish(topic string, qos byte, retained bool, payload any) error
}

// detectionDedupWindow is how long the same drone+heading is suppressed from DB writes.
const detectionDedupWindow = 20 * time.Second

type detectionCacheEntry struct {
	targetName  string
	heading     int
	lastSavedAt time.Time
}

var (
	detectionCacheMu sync.Mutex
	detectionCache   = map[string]detectionCacheEntry{} // key: uniqueID
)

// MonitoringData represents a heartbeat/status frame from the drone detector.
type MonitoringData struct {
	DeviceID         int32
	DeviceName       string
	Longitude        float32
	Latitude         float32
	Altitude         int32
	OpStatus         int16
	Azimuth          float32
	DeviceType       string
	CompassStatus    int8
	GPSStatus        int8
	RFSwitchStatus   int8
	ConnectionStatus int8
	CoverageArea     int32
	RecvDeviceID     int32
	Temperature      float32
	Humidity         float32
}

// DroneData represents a detected drone target frame.
type DroneData struct {
	UniqueID       string
	TargetID       int32
	TargetName     string
	DroneLongitude float32
	DroneLatitude  float32
	DroneAltitude  int32
	BaroAltitude   int32
	DirectionAngle int32
	Distance       int32
	RemoteLong     float32
	RemoteLat      float32
	Frequency      float64
	Bandwidth      float64
	SignalStrength float64
	Confidence     uint8
	Timestamp      uint32
	FlightSpeed    float64
}

// StartDroneDetector connects to a drone detector device via TCP and parses its binary protocol.
// It reconnects automatically with exponential backoff on connection failures.
func StartDroneDetector(label, host string, port int, lat, lng float64) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	log.Printf("[%s] detector location: %.6f, %.6f (from database)", label, lat, lng)
	registerDetectorLocation(label, lat, lng)
	backoff := 5 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		log.Printf("[%s] connecting to drone detector at %s...", label, addr)

		conn, err := net.DialTimeout("tcp4", addr, 10*time.Second)
		if err != nil {
			log.Printf("[%s] connection failed: %v, retrying in %s...", label, err, backoff)
			reportDetectorStatus(host, port, "offline")
			time.Sleep(backoff)
			// Increase backoff for next failure, capped at maxBackoff
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Connected — reset backoff
		backoff = 5 * time.Second
		log.Printf("[%s] connected to %s", label, addr)
		reportDetectorStatus(host, port, "online")
		handleConnection(label, conn)
		log.Printf("[%s] disconnected from %s, reconnecting in 5s...", label, addr)
		reportDetectorStatus(host, port, "offline")
		time.Sleep(5 * time.Second)
	}
}

func handleConnection(label string, conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 0, 65536)
	readBuf := make([]byte, 4096)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(readBuf)
		if err != nil {
			log.Printf("[%s] read error: %v", label, err)
			return
		}

		buf = append(buf, readBuf[:n]...)
		buf = processBuffer(label, buf)
	}
}

func processBuffer(label string, buf []byte) []byte {
	for len(buf) >= 34 {
		// Find start flag 0xEEEEEEEE
		idx := findStartFlag(buf)
		if idx == -1 {
			// Keep the last 3 bytes in case they are a partial start flag
			if len(buf) > 3 {
				return buf[len(buf)-3:]
			}
			return buf
		}
		if idx > 0 {
			buf = buf[idx:]
		}

		if len(buf) < 10 {
			break
		}

		// Frame length at offset 6 (uint32 LE)
		frameLength := int(binary.LittleEndian.Uint32(buf[6:10]))
		if frameLength < 34 || frameLength > 65536 {
			// Invalid frame length, skip this start flag
			buf = buf[4:]
			continue
		}

		if len(buf) < frameLength {
			break // Wait for more data
		}

		frame := buf[:frameLength]

		// Verify end flag 0xAAAAAAAA at last 4 bytes
		endFlagOffset := frameLength - 4
		endFlag := binary.LittleEndian.Uint32(frame[endFlagOffset:])
		if endFlag == 0xAAAAAAAA {
			parseFrame(label, frame)
		} else {
			log.Printf("[%s] end flag mismatch, dropping frame", label)
		}

		buf = buf[frameLength:]
	}
	return buf
}

func findStartFlag(buf []byte) int {
	for i := 0; i <= len(buf)-4; i++ {
		if buf[i] == 0xEE && buf[i+1] == 0xEE && buf[i+2] == 0xEE && buf[i+3] == 0xEE {
			return i
		}
	}
	return -1
}

func parseFrame(label string, frame []byte) {
	// Data type at offset 19 (uint16 LE)
	dataType := binary.LittleEndian.Uint16(frame[19:21])
	// Data section: offset 29 to (length - 5)
	dataSection := frame[29 : len(frame)-5]

	switch dataType {
	case 1:
		parseMonitoringData(label, dataSection)
	case 56:
		parseDroneData(label, dataSection)
	default:
		log.Printf("[%s] unknown data type: %d (%d bytes)", label, dataType, len(dataSection))
	}
}

func parseMonitoringData(label string, data []byte) {
	if len(data) < 74 {
		return
	}

	m := MonitoringData{
		DeviceID:         int32(binary.LittleEndian.Uint32(data[0:4])),
		DeviceName:       trimNull(string(data[4:24])),
		Longitude:        math.Float32frombits(binary.LittleEndian.Uint32(data[24:28])),
		Latitude:         math.Float32frombits(binary.LittleEndian.Uint32(data[28:32])),
		Altitude:         int32(binary.LittleEndian.Uint32(data[32:36])),
		OpStatus:         int16(binary.LittleEndian.Uint16(data[36:38])),
		Azimuth:          math.Float32frombits(binary.LittleEndian.Uint32(data[38:42])),
		DeviceType:       trimNull(string(data[42:46])),
		CompassStatus:    int8(data[46]),
		GPSStatus:        int8(data[47]),
		RFSwitchStatus:   int8(data[48]),
		ConnectionStatus: int8(data[49]),
		CoverageArea:     int32(binary.LittleEndian.Uint32(data[50:54])),
		RecvDeviceID:     int32(binary.LittleEndian.Uint32(data[54:58])),
		Temperature:      math.Float32frombits(binary.LittleEndian.Uint32(data[58:62])),
		Humidity:         math.Float32frombits(binary.LittleEndian.Uint32(data[62:66])),
	}

	statusStr := "Idle"
	if m.OpStatus == 1 {
		statusStr = "Working"
	}
	compassStr := statusLabel(m.CompassStatus >= 0, "Normal", "Abnormal")
	gpsStr := statusLabel(m.GPSStatus >= 0, "Normal", "Abnormal")
	rfStr := statusLabel(m.RFSwitchStatus == 1, "Normal", "Off")
	connStr := "Connected"
	if m.ConnectionStatus == -1 {
		connStr = "Initializing"
	} else if m.ConnectionStatus == 1 {
		connStr = "Disconnected"
	}

	log.Printf("[%s] === DEVICE HEARTBEAT ===", label)
	log.Printf("[%s]   Device ID:    %d", label, m.DeviceID)
	log.Printf("[%s]   Name:         %s", label, m.DeviceName)
	log.Printf("[%s]   Type:         %s", label, m.DeviceType)
	log.Printf("[%s]   Location:     %.6f, %.6f", label, m.Latitude, m.Longitude)
	log.Printf("[%s]   Altitude:     %d m", label, m.Altitude)
	log.Printf("[%s]   Azimuth:      %.1f°", label, m.Azimuth)
	log.Printf("[%s]   Coverage:     %d°", label, m.CoverageArea)
	log.Printf("[%s]   Temperature:  %.2f °C", label, m.Temperature)
	log.Printf("[%s]   Humidity:     %.2f %%", label, m.Humidity)
	log.Printf("[%s]   Status:       %s", label, statusStr)
	log.Printf("[%s]   Compass:      %s | GPS: %s | RF: %s | Antenna: %s",
		label, compassStr, gpsStr, rfStr, connStr)
}

func parseDroneData(label string, data []byte) {
	if len(data) < 24 {
		return
	}

	uniqueID := trimNull(string(data[0:16]))
	targetID := int32(binary.LittleEndian.Uint32(data[16:20]))
	nameLength := int(binary.LittleEndian.Uint32(data[20:24]))

	if len(data) < 24+nameLength+69 {
		log.Printf("[%s] incomplete drone data frame", label)
		return
	}

	targetName := trimNull(string(data[24 : 24+nameLength]))
	off := 24 + nameLength

	d := DroneData{
		UniqueID:       uniqueID,
		TargetID:       targetID,
		TargetName:     targetName,
		DroneLongitude: math.Float32frombits(binary.LittleEndian.Uint32(data[off : off+4])),
		DroneLatitude:  math.Float32frombits(binary.LittleEndian.Uint32(data[off+4 : off+8])),
		DroneAltitude:  int32(binary.LittleEndian.Uint32(data[off+8 : off+12])),
		BaroAltitude:   int32(binary.LittleEndian.Uint32(data[off+12 : off+16])),
		DirectionAngle: int32(binary.LittleEndian.Uint32(data[off+16 : off+20])),
		Distance:       int32(binary.LittleEndian.Uint32(data[off+20 : off+24])),
		RemoteLong:     math.Float32frombits(binary.LittleEndian.Uint32(data[off+24 : off+28])),
		RemoteLat:      math.Float32frombits(binary.LittleEndian.Uint32(data[off+28 : off+32])),
		Frequency:      math.Float64frombits(binary.LittleEndian.Uint64(data[off+32 : off+40])),
		Bandwidth:      math.Float64frombits(binary.LittleEndian.Uint64(data[off+40 : off+48])),
		SignalStrength: math.Float64frombits(binary.LittleEndian.Uint64(data[off+48 : off+56])),
		Confidence:     data[off+56],
		Timestamp:      binary.LittleEndian.Uint32(data[off+57 : off+61]),
		FlightSpeed:    math.Float64frombits(binary.LittleEndian.Uint64(data[off+61 : off+69])),
	}

	// LOGIC MARK: Detection processing starts here. No frequency filter is applied.

	// Quality filter: drop frames that don't meet the configured thresholds.
	detectionFilterMu.RLock()
	confThresh := detectionMinConfidence
	sigThresh := detectionMinSignalStrength
	detectionFilterMu.RUnlock()
	if confThresh > 0 && d.Confidence < confThresh {
		log.Printf("[%s] filtered: confidence %d%% < threshold %d%% (target=%s)", label, d.Confidence, confThresh, d.TargetName)
		return
	}
	if sigThresh < 0 && d.SignalStrength < sigThresh {
		log.Printf("[%s] filtered: signal %.2f dB < threshold %.2f dB (target=%s)", label, d.SignalStrength, sigThresh, d.TargetName)
		return
	}

	// Deduplication: skip DB write only when name AND heading are unchanged
	// AND the last saved write for this drone was less than detectionDedupWindow ago.
	// If the same drone is seen continuously for >= detectionDedupWindow, the next
	// detection is saved again (and the window restarts).
	now := time.Now()
	detectionCacheMu.Lock()
	entry, exists := detectionCache[d.UniqueID]
	shouldSave := !exists ||
		entry.targetName != d.TargetName ||
		entry.heading != int(d.DirectionAngle) ||
		now.Sub(entry.lastSavedAt) >= detectionDedupWindow
	if shouldSave {
		detectionCache[d.UniqueID] = detectionCacheEntry{
			targetName:  d.TargetName,
			heading:     int(d.DirectionAngle),
			lastSavedAt: now,
		}
	}
	detectionCacheMu.Unlock()

	log.Printf("[%s] === TARGET IDENTIFIED: %s ===", label, d.TargetName)
	log.Printf("[%s]   Unique ID:    %s", label, d.UniqueID)
	log.Printf("[%s]   Target ID:    %d", label, d.TargetID)
	log.Printf("[%s]   Drone:        %.6f, %.6f  Alt: %d m  Baro: %d m",
		label, d.DroneLatitude, d.DroneLongitude, d.DroneAltitude, d.BaroAltitude)
	log.Printf("[%s]   Heading:      %d°  Distance: %d m  Speed: %.2f m/s",
		label, d.DirectionAngle, d.Distance, d.FlightSpeed)
	log.Printf("[%s]   Remote:       %.6f, %.6f",
		label, d.RemoteLat, d.RemoteLong)
	log.Printf("[%s]   RF:           Freq: %.0f kHz  BW: %.0f kHz  Signal: %.2f dB",
		label, d.Frequency, d.Bandwidth, d.SignalStrength)
	log.Printf("[%s]   Confidence:   %d%%  Timestamp: %d",
		label, d.Confidence, d.Timestamp)

	// Always publish to MQTT for live feed on detections page.
	go publishDetectionToMQTT(label, d, shouldSave)

	// LOGIC MARK: This line triggers the write to the database (via API POST)
	if shouldSave {
		go postDroneEvent(label, d)
	} else {
		log.Printf("[%s] dedup: skipping DB write for %s (same name+heading within %s)",
			label, d.UniqueID, detectionDedupWindow)
	}

	// LOGIC MARK: This triggers automatic blocker activation based on detection
	autoFlagsMu.RLock()
	blockerEnabled := autoBlockerEnabled
	cameraEnabled := autoCameraEnabled
	autoFlagsMu.RUnlock()
	if blockerEnabled {
		go autoActivateBlockers(label, d)
	}

	// LOGIC MARK: This rotates configured cameras toward the detected drone
	if cameraEnabled {
		go autoTrackCamera(label, d)
	}
}

func trimNull(s string) string {
	for i, c := range s {
		if c == 0 {
			return s[:i]
		}
	}
	return s
}

func statusLabel(ok bool, good, bad string) string {
	if ok {
		return good
	}
	return bad
}

// publishDetectionToMQTT publishes every drone detection to MQTT topic detections/live.
// saved=true means this detection was also written to the database; saved=false means it was deduped.
func publishDetectionToMQTT(label string, d DroneData, saved bool) {
	if mqttDetectPublisher == nil {
		return
	}
	payload := map[string]any{
		"detector":        label,
		"unique_id":       d.UniqueID,
		"target_name":     d.TargetName,
		"drone_lat":       float64(d.DroneLatitude),
		"drone_lng":       float64(d.DroneLongitude),
		"drone_alt":       int(d.DroneAltitude),
		"heading":         int(d.DirectionAngle),
		"distance":        int(d.Distance),
		"speed":           d.FlightSpeed,
		"frequency":       d.Frequency,
		"signal_strength": d.SignalStrength,
		"confidence":      int(d.Confidence),
		"saved":           saved,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if err := mqttDetectPublisher.Publish("detections/live", 0, false, body); err != nil {
		log.Printf("[%s] failed to publish detection to MQTT: %v", label, err)
	}
}

// postDroneEvent sends a detected drone event to the backend API.
func postDroneEvent(label string, d DroneData) {
	payload := map[string]any{
		"detector":        label,
		"unique_id":       d.UniqueID,
		"target_name":     d.TargetName,
		"drone_lat":       float64(d.DroneLatitude),
		"drone_lng":       float64(d.DroneLongitude),
		"drone_alt":       int(d.DroneAltitude),
		"heading":         int(d.DirectionAngle),
		"distance":        int(d.Distance),
		"speed":           d.FlightSpeed,
		"frequency":       d.Frequency,
		"signal_strength": d.SignalStrength,
		"confidence":      d.Confidence,
		"remote_lat":      float64(d.RemoteLat),
		"remote_lng":      float64(d.RemoteLong),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[%s] failed to marshal drone event: %v", label, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", backendURL+"/api/drone-events", bytes.NewReader(body))
	if err != nil {
		log.Printf("[%s] failed to create drone event request: %v", label, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] failed to post drone event: %v", label, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Printf("[%s] drone event POST returned status %d", label, resp.StatusCode)
	}
}

// blockerRule maps a heading range to a list of dblocker serial numbers to preset-ON.
// headingMax is exclusive on the upper bound.
type blockerRule struct {
	headingMin     int
	headingMax     int
	blockerSerials []string
}

// ============================================================
// MAPPING RULES — EDIT THIS SECTION TO CONFIGURE ACTIVATION
//
// Syntax:
//
//		<detectorID>: {
//		    {headingMin: <from°>, headingMax: <to°>, blockerSerials: []string{"<serial>", ...}},
//		    ...
//		},
//
//	  - detectorID    : Numeric ID of the drone detector (from the database).
//	    Use the ID — NOT the name — so rules survive detector renames.
//	  - headingMin    : start of heading range (inclusive), 0–359
//	  - headingMax    : end of heading range (exclusive), 1–360
//	  - blockerSerials: one or more dblocker serial numbers to preset-ON when matched
//
// Using serial_numb instead of ID makes rules resilient to dblocker re-registration.
//
// Example: detector ID 1, heading 0–89°   → activate dblocker serial "250001"
//
//	detector ID 1, heading 270–359° → activate dblockers "250001"…"250005"
//	detector ID 2, heading 0–89°   → activate dblockers "250008" AND "250009"
//
// ============================================================
var detectorRules = map[int][]blockerRule{
	1: {
		{headingMin: 0, headingMax: 90, blockerSerials: []string{"250006", "250007"}},
		{headingMin: 270, headingMax: 360, blockerSerials: []string{"250001", "250002", "250003", "250004", "250005"}},
	},
	2: {
		{headingMin: 0, headingMax: 90, blockerSerials: []string{"250008", "250009"}},
		{headingMin: 90, headingMax: 180, blockerSerials: []string{"250009", "250010"}},
	},
}

// ============================================================

// dblockerSerialCache caches serial_numb → dblocker ID to avoid repeated API calls.
var (
	dblockerSerialCache   map[string]uint
	dblockerSerialCacheMu sync.RWMutex
)

// resolveBlockerID returns the dblocker ID for a given serial number.
// It uses a cache and refreshes it on a cache miss.
func resolveBlockerID(label, serial string) (uint, bool) {
	dblockerSerialCacheMu.RLock()
	id, ok := dblockerSerialCache[serial]
	dblockerSerialCacheMu.RUnlock()
	if ok {
		return id, true
	}
	// Cache miss — refresh from API
	if err := refreshBlockerCache(); err != nil {
		log.Printf("[%s] resolveBlockerID: failed to refresh cache: %v", label, err)
		return 0, false
	}
	dblockerSerialCacheMu.RLock()
	id, ok = dblockerSerialCache[serial]
	dblockerSerialCacheMu.RUnlock()
	if !ok {
		log.Printf("[%s] resolveBlockerID: serial %q not found in dblocker list", label, serial)
	}
	return id, ok
}

// refreshBlockerCache fetches all dblockers from the API and rebuilds the serial→ID cache.
func refreshBlockerCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backendURL+"/api/dblockers", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID         uint   `json:"id"`
			SerialNumb string `json:"serial_numb"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	newCache := make(map[string]uint, len(result.Data))
	for _, b := range result.Data {
		newCache[b.SerialNumb] = b.ID
	}

	dblockerSerialCacheMu.Lock()
	dblockerSerialCache = newCache
	dblockerSerialCacheMu.Unlock()
	return nil
}

// autoActivateBlockers applies the detector→heading→dblocker mapping rules
// and fires preset-ON for each matched dblocker.
func autoActivateBlockers(label string, d DroneData) {
	// Check whitelist — whitelisted drones are observed but never trigger blockers.
	whitelistMu.RLock()
	whitelisted := (d.UniqueID != "" && whitelistUniqueIDs[d.UniqueID]) ||
		(d.TargetName != "" && whitelistTargetNames[d.TargetName])
	whitelistMu.RUnlock()
	if whitelisted {
		log.Printf("[%s] drone whitelisted (uid=%q target=%q) — skipping blocker activation", label, d.UniqueID, d.TargetName)
		return
	}

	// Parse detector ID from label format "detector-{id}-{name}"
	parts := strings.SplitN(label, "-", 3)
	if len(parts) < 3 {
		log.Printf("[%s] autoActivateBlockers: cannot parse detector ID from label", label)
		return
	}
	detectorID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("[%s] autoActivateBlockers: invalid detector ID %q in label", label, parts[1])
		return
	}

	rules, ok := detectorRules[detectorID]
	if !ok {
		return // no rules configured for this detector
	}

	heading := int(d.DirectionAngle)

	for _, rule := range rules {
		if heading >= rule.headingMin && heading < rule.headingMax {
			for _, serial := range rule.blockerSerials {
				log.Printf("[%s] heading %d° matches rule [%d-%d°) → activating dblocker serial %q preset",
					label, heading, rule.headingMin, rule.headingMax, serial)
				go scheduleBlockerOff(label, serial)
			}
		}
	}
}

// applyBlockerPreset resolves a dblocker serial number to its ID and calls the preset-ON endpoint.
func applyBlockerPreset(label, serial string) {
	blockerID, ok := resolveBlockerID(label, serial)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/dblockers/config/preset/%d", backendURL, blockerID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[%s] applyBlockerPreset: failed to create request for dblocker %q: %v", label, serial, err)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] applyBlockerPreset: request failed for dblocker %q: %v", label, serial, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] applyBlockerPreset: dblocker %q returned status %d", label, serial, resp.StatusCode)
	} else {
		log.Printf("[%s] ✅ PRESET-ON sent to dblocker serial=%q (id=%d)", label, serial, blockerID)
	}
}

// scheduleBlockerOff activates a dblocker preset and sets (or resets) its auto-off hold timer.
func scheduleBlockerOff(label, serial string) {
	// Activate (or keep active) the blocker preset.
	go applyBlockerPreset(label, serial)

	holdSecondsMu.RLock()
	dur := time.Duration(holdSeconds) * time.Second
	holdSecondsMu.RUnlock()

	holdTimersMu.Lock()
	defer holdTimersMu.Unlock()

	if t, ok := holdTimers[serial]; ok {
		// Detection came in before timer fired — stop the old timer and replace
		// it with a fresh one so a racing AfterFunc callback won't fire twice.
		t.Stop()
		delete(holdTimers, serial)
	}

	// Create a new timer. Capture it in a local var so the callback can verify
	// the map still points to *this* timer before triggering the off-action;
	// otherwise we may turn the blocker off prematurely after a Reset race.
	var newTimer *time.Timer
	newTimer = time.AfterFunc(dur, func() {
		holdTimersMu.Lock()
		current, ok := holdTimers[serial]
		if !ok || current != newTimer {
			// A newer scheduling has replaced us; do nothing.
			holdTimersMu.Unlock()
			return
		}
		delete(holdTimers, serial)
		holdTimersMu.Unlock()
		log.Printf("[%s] hold timer expired for dblocker %q - restoring default config", label, serial)
		// go turnOffBlocker(label, serial)
		go applyBlockerDefault(label, serial) // uncomment to restore default config instead of turning off
	})
	holdTimers[serial] = newTimer
}

// applyBlockerDefault resolves a dblocker serial number to its ID and calls the default-ON endpoint.
func applyBlockerDefault(label, serial string) {
	blockerID, ok := resolveBlockerID(label, serial)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/dblockers/config/default/%d", backendURL, blockerID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[%s] applyBlockerDefault: failed to create request for dblocker %q: %v", label, serial, err)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] applyBlockerDefault: request failed for dblocker %q: %v", label, serial, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		// Device is offline — queue for retry when it comes back online.
		log.Printf("[%s] applyBlockerDefault: dblocker %q is offline, queuing for retry", label, serial)
		pendingRestoresMu.Lock()
		pendingRestores[serial] = label
		pendingRestoresMu.Unlock()
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] applyBlockerDefault: dblocker %q returned status %d", label, serial, resp.StatusCode)
	} else {
		log.Printf("[%s] ✅ DEFAULT-ON sent to dblocker serial=%q (id=%d) after hold timer", label, serial, blockerID)
		// Clear any pending retry for this serial since it just succeeded.
		pendingRestoresMu.Lock()
		delete(pendingRestores, serial)
		pendingRestoresMu.Unlock()
	}
}

// turnOffBlocker calls the all-off endpoint for a dblocker by serial number.
func turnOffBlocker(label, serial string) {
	blockerID, ok := resolveBlockerID(label, serial)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/dblockers/config/off/%d", backendURL, blockerID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[%s] turnOffBlocker: failed to create request for dblocker %q: %v", label, serial, err)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] turnOffBlocker: request failed for dblocker %q: %v", label, serial, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] turnOffBlocker: dblocker %q returned status %d", label, serial, resp.StatusCode)
	} else {
		log.Printf("[%s] ✅ ALL-OFF sent to dblocker serial=%q (id=%d) after hold timer", label, serial, blockerID)
	}
}

// startHoldSettingSync polls the backend every 5 seconds and updates holdSeconds.
func startHoldSettingSync() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			fetchAndUpdateHoldSeconds()
		}
	}()
}

// startPendingRestoreRetry retries default-config restores that failed because
// the device was offline when the hold timer fired. Runs every 5 seconds.
func startPendingRestoreRetry() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			pendingRestoresMu.Lock()
			if len(pendingRestores) == 0 {
				pendingRestoresMu.Unlock()
				continue
			}
			// Snapshot and clear; applyBlockerDefault will re-add if still offline.
			snap := make(map[string]string, len(pendingRestores))
			for s, l := range pendingRestores {
				snap[s] = l
			}
			for s := range pendingRestores {
				delete(pendingRestores, s)
			}
			pendingRestoresMu.Unlock()
			for serial, label := range snap {
				log.Printf("[%s] retrying pending default restore for dblocker %q", label, serial)
				go applyBlockerDefault(label, serial)
			}
		}
	}()
}

// startDetectionCacheCleanup periodically purges stale entries from
// detectionCache to prevent unbounded memory growth from unique drone IDs
// that are seen once and never again.
func startDetectionCacheCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-10 * time.Minute)
			detectionCacheMu.Lock()
			for id, e := range detectionCache {
				if e.lastSavedAt.Before(cutoff) {
					delete(detectionCache, id)
				}
			}
			detectionCacheMu.Unlock()
		}
	}()
}

func fetchAndUpdateHoldSeconds() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backendURL+"/api/detectors/settings", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Only trust the response if the API call actually succeeded.
	// Otherwise an error body could be decoded into zero-value (false) flags
	// and silently disable auto-blocker / auto-camera.
	if resp.StatusCode != http.StatusOK {
		log.Printf("warn: detector settings poll returned status %d, keeping previous flags", resp.StatusCode)
		return
	}

	var result struct {
		Data struct {
			HoldSeconds       int     `json:"hold_seconds"`
			AutoBlocker       bool    `json:"auto_blocker"`
			AutoCamera        bool    `json:"auto_camera"`
			MinConfidence     uint8   `json:"min_confidence"`
			MinSignalStrength float64 `json:"min_signal_strength"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}
	if result.Data.HoldSeconds >= 5 {
		holdSecondsMu.Lock()
		holdSeconds = result.Data.HoldSeconds
		holdSecondsMu.Unlock()
	}
	autoFlagsMu.Lock()
	autoBlockerEnabled = result.Data.AutoBlocker
	autoCameraEnabled = result.Data.AutoCamera
	autoFlagsMu.Unlock()
	detectionFilterMu.Lock()
	detectionMinConfidence = result.Data.MinConfidence
	detectionMinSignalStrength = result.Data.MinSignalStrength
	detectionFilterMu.Unlock()

	fetchAndUpdateWhitelist()
}

// fetchAndUpdateWhitelist pulls the current drone whitelist from the backend
// and refreshes the in-memory cache used by autoActivateBlockers.
func fetchAndUpdateWhitelist() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backendURL+"/api/whitelist", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("warn: whitelist poll returned status %d, keeping previous list", resp.StatusCode)
		return
	}

	var result struct {
		Data []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	newUIDs := map[string]bool{}
	newTargets := map[string]bool{}
	for _, entry := range result.Data {
		switch entry.Type {
		case "unique_id":
			newUIDs[entry.Value] = true
		case "target_name":
			newTargets[entry.Value] = true
		}
	}

	whitelistMu.Lock()
	whitelistUniqueIDs = newUIDs
	whitelistTargetNames = newTargets
	whitelistMu.Unlock()
}

// calcBearing returns the bearing in degrees (0-360) from point A to point B.
func calcBearing(lat1, lng1, lat2, lng2 float64) float64 {
	lat1r := lat1 * math.Pi / 180
	lat2r := lat2 * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	y := math.Sin(dLng) * math.Cos(lat2r)
	x := math.Cos(lat1r)*math.Sin(lat2r) - math.Sin(lat1r)*math.Cos(lat2r)*math.Cos(dLng)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	if bearing < 0 {
		bearing += 360
	}
	return bearing
}

// reportDetectorStatus sends a status update (online/offline) to the backend.
func reportDetectorStatus(host string, port int, status string) {
	payload := map[string]any{
		"host":   host,
		"port":   port,
		"status": status,
	}
	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "PUT", backendURL+"/api/detectors/status", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("warn: failed to report detector status: %v", err)
		return
	}
	defer resp.Body.Close()
}
