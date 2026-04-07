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
func StartDroneDetector(label, host string, port int) {
	addr := fmt.Sprintf("%s:%d", host, port)
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

	// Auto-activate blockers based on drone position
	go autoActivateBlockers(label, d)
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

// autoActivateBlockers determines which blocker sectors to activate based on
// the bearing from each blocker to the detected drone. For each blocker, it
// calculates the angle to the drone and maps it to the correct 60° sector
// (adjusted by the blocker's angle_start). It then turns on both GPS and Ctrl
// signals for that sector.
func autoActivateBlockers(label string, d DroneData) {
	if d.DroneLatitude == 0 && d.DroneLongitude == 0 {
		return // No valid position data
	}

	// Fetch all dblockers
	req, err := http.NewRequest("GET", backendURL+"/api/dblockers", nil)
	if err != nil {
		log.Printf("[%s] auto-activate: failed to create request: %v", label, err)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] auto-activate: failed to fetch dblockers: %v", label, err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID         uint             `json:"id"`
			Name       string           `json:"name"`
			Lat        float64          `json:"latitude"`
			Lng        float64          `json:"longitude"`
			AngleStart int              `json:"angle_start"`
			Config     []DBlockerConfig `json:"config"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[%s] auto-activate: failed to decode dblockers: %v", label, err)
		return
	}

	droneLat := float64(d.DroneLatitude)
	droneLng := float64(d.DroneLongitude)

	for _, blocker := range result.Data {
		// Calculate bearing from blocker to drone
		bearing := calcBearing(blocker.Lat, blocker.Lng, droneLat, droneLng)

		// Adjust bearing by blocker's angle_start offset
		adjusted := bearing - float64(blocker.AngleStart)
		if adjusted < 0 {
			adjusted += 360
		}

		// Map to sector index (each sector covers 60°)
		sectorIdx := int(adjusted/60) % 6

		// Build config: activate the target sector (both GPS & Ctrl ON)
		var config [6]DBlockerConfig
		// Preserve existing active sectors
		for i := 0; i < 6 && i < len(blocker.Config); i++ {
			config[i] = blocker.Config[i]
		}
		config[sectorIdx] = DBlockerConfig{SignalGPS: true, SignalCtrl: true}

		log.Printf("[%s] auto-activate: blocker %q (ID=%d) bearing=%.1f° adjusted=%.1f° → sector %d ON",
			label, blocker.Name, blocker.ID, bearing, adjusted, sectorIdx)

		// Apply config
		update := ConfigUpdatePayload{ID: blocker.ID, Config: config}
		body, _ := json.Marshal(update)
		putReq, err := http.NewRequest("PUT", backendURL+"/api/dblockers/config", bytes.NewReader(body))
		if err != nil {
			log.Printf("[%s] auto-activate: request error: %v", label, err)
			continue
		}
		putReq.Header.Set("Content-Type", "application/json")
		putReq.Header.Set("X-API-Key", apiKey)

		putResp, err := client.Do(putReq)
		if err != nil {
			log.Printf("[%s] auto-activate: PUT error: %v", label, err)
			continue
		}
		putResp.Body.Close()

		// Log the auto action
		logPayload := ActionLogPayload{
			Username:     fmt.Sprintf("auto[%s]", label),
			Action:       "auto_drone_response",
			DBlockerID:   blocker.ID,
			DBlockerName: blocker.Name,
			Config:       config[:],
		}
		logBody, _ := json.Marshal(logPayload)
		logReq, _ := http.NewRequest("POST", backendURL+"/api/logs", bytes.NewReader(logBody))
		logReq.Header.Set("Content-Type", "application/json")
		logReq.Header.Set("X-API-Key", apiKey)
		logResp, err := client.Do(logReq)
		if err == nil {
			logResp.Body.Close()
		}
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
