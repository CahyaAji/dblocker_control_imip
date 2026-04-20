package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var startFlag = []byte{0xEE, 0xEE, 0xEE, 0xEE}

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
	addr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("[%s] detector location: %.6f, %.6f (from database)", label, lat, lng)
	backoff := 5 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		log.Printf("[%s] connecting to drone detector at %s...", label, addr)

		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
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
			return buf[:0]
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

	// Post drone event to backend
	go postDroneEvent(label, d)

	// Auto-activate blockers based on drone position (disabled)
	// go autoActivateBlockers(label, d)
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

// postDroneEvent sends a detected drone event to the backend API.
func postDroneEvent(label string, d DroneData) {
	payload := map[string]any{
		"detector":    label,
		"unique_id":   d.UniqueID,
		"target_name": d.TargetName,
		"drone_lat":   float64(d.DroneLatitude),
		"drone_lng":   float64(d.DroneLongitude),
		"drone_alt":   int(d.DroneAltitude),
		"heading":     int(d.DirectionAngle),
		"distance":    int(d.Distance),
		"speed":       d.FlightSpeed,
		"frequency":   d.Frequency,
		"confidence":  d.Confidence,
		"remote_lat":  float64(d.RemoteLat),
		"remote_lng":  float64(d.RemoteLong),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[%s] failed to marshal drone event: %v", label, err)
		return
	}

	req, err := http.NewRequest("POST", backendURL+"/api/drone-events", bytes.NewReader(body))
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

	if resp.StatusCode != http.StatusCreated {
		log.Printf("[%s] drone event POST returned status %d", label, resp.StatusCode)
	}
}

// detectorBlockerRules maps (detectorName, headingMin, headingMax) to a list of dblocker serial numbers to preset-ON.
// headingMax is exclusive on the upper bound and the range wraps at 360.
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
//	"<detectorName>": {
//	    {headingMin: <from°>, headingMax: <to°>, blockerSerials: []string{"<serial>", ...}},
//	    ...
//	},
//
// - detectorName  : Name of the drone detector (from the database)
// - headingMin    : start of heading range (inclusive), 0–359
// - headingMax    : end of heading range (exclusive), 1–360
// - blockerSerials: one or more dblocker serial numbers to preset-ON when matched
//
// Using serial_numb instead of ID makes rules resilient to device re-registration.
//
// Example: "Detector 1", heading 0–119°  → activate dblocker with serial "250001"
//
//	"Detector 1", heading 120–239° → activate dblocker with serial "250003"
//	"Detector 2", heading 0–89°   → activate dblockers "250001" AND "250002"
//
// ============================================================
var detectorRules = map[string][]blockerRule{
	"Detector 1": {
		{headingMin: 0, headingMax: 120, blockerSerials: []string{"250001"}},
		{headingMin: 120, headingMax: 240, blockerSerials: []string{"250003"}},
	},
	"Detector 2": {
		{headingMin: 0, headingMax: 90, blockerSerials: []string{"250001", "250002"}},
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
	req, err := http.NewRequest("GET", backendURL+"/api/dblockers", nil)
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
	// Parse detector name from label format "detector-{id}-{name}"
	parts := strings.SplitN(label, "-", 3)
	if len(parts) < 3 {
		log.Printf("[%s] autoActivateBlockers: cannot parse detector name from label", label)
		return
	}
	detectorName := parts[2]

	rules, ok := detectorRules[detectorName]
	if !ok {
		return // no rules configured for this detector
	}

	heading := int(d.DirectionAngle)

	for _, rule := range rules {
		if heading >= rule.headingMin && heading < rule.headingMax {
			for _, serial := range rule.blockerSerials {
				log.Printf("[%s] heading %d° matches rule [%d-%d°) → activating dblocker serial %q preset",
					label, heading, rule.headingMin, rule.headingMax, serial)
				go applyBlockerPreset(label, serial)
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
	req, err := http.NewRequest("GET", url, nil)
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
	}
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

	req, err := http.NewRequest("PUT", backendURL+"/api/detectors/status", bytes.NewReader(body))
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
	resp.Body.Close()
}
