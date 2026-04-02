package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
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
func StartDroneDetector(label, host string, port int) {
	addr := fmt.Sprintf("%s:%d", host, port)

	for {
		log.Printf("[%s] connecting to drone detector at %s...", label, addr)

		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			log.Printf("[%s] connection failed: %v, retrying in 5s...", label, err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("[%s] connected to %s", label, addr)
		handleConnection(label, conn)
		log.Printf("[%s] disconnected, reconnecting in 5s...", label, addr)
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
