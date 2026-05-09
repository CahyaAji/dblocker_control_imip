package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

// ── Configuration globals (set from main) ────────────────────────────────────

// visionURL is the base URL of the vision server.
var visionURL string

// droneRangeFallbackM is the assumed horizontal distance (metres) from the
// detector to the drone when the drone reports lat=0 lng=0 (no GPS fix).
var droneRangeFallbackM float64 = 1000

// cameraHeightM is the assumed height (metres above ground) of the camera mount.
var cameraHeightM float64 = 20

// ── Camera track rules ───────────────────────────────────────────────────────

// cameraTrackRule maps a detector name to one or more vision-server device IDs.
// Multiple devices can be listed if you want more than one camera to track.
//
// ============================================================
// EDIT THIS SECTION TO CONFIGURE WHICH CAMERAS TRACK WHICH DETECTORS
//
//	"Detector 1": {1}   → detector named "Detector 1" moves camera device ID 1
//	"Detector 2": {2}   → detector named "Detector 2" moves camera device ID 2
//
// ============================================================
var cameraTrackRules = map[string][]int{
	"Detector1": {1},
	"Detector2": {2},
}

// northOffsets maps vision-server device ID → compass bearing (degrees) that
// the camera faces when ISAPI azimuth = 0.
// Set via env DEVICE_1_NORTH_OFFSET, DEVICE_2_NORTH_OFFSET, etc. (default 0).
var northOffsets = map[int]float64{}

// ── Detector location cache ───────────────────────────────────────────────────

// detectorLocations stores lat/lng for each detector label, populated when the
// detector goroutine starts.
var (
	detectorLocationsMu sync.RWMutex
	detectorLocations   = map[string][2]float64{} // label → [lat, lng]
)

// registerDetectorLocation stores the known GPS position of a detector by label.
func registerDetectorLocation(label string, lat, lng float64) {
	detectorLocationsMu.Lock()
	detectorLocations[label] = [2]float64{lat, lng}
	detectorLocationsMu.Unlock()
}

// ── Camera device cache ───────────────────────────────────────────────────────

type cameraDeviceInfo struct {
	ID  int     `json:"id"`
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

var (
	cameraDeviceCacheMu sync.RWMutex
	cameraDeviceCache   []cameraDeviceInfo
)

func getCameraDevice(deviceID int) (cameraDeviceInfo, bool) {
	cameraDeviceCacheMu.RLock()
	for _, d := range cameraDeviceCache {
		if d.ID == deviceID {
			cameraDeviceCacheMu.RUnlock()
			return d, true
		}
	}
	cameraDeviceCacheMu.RUnlock()

	// Cache miss — refresh from vision server.
	if err := refreshCameraDeviceCache(); err != nil {
		log.Printf("camera_track: failed to refresh camera device cache: %v", err)
		return cameraDeviceInfo{}, false
	}

	cameraDeviceCacheMu.RLock()
	defer cameraDeviceCacheMu.RUnlock()
	for _, d := range cameraDeviceCache {
		if d.ID == deviceID {
			return d, true
		}
	}
	log.Printf("camera_track: device ID %d not found in vision server", deviceID)
	return cameraDeviceInfo{}, false
}

func refreshCameraDeviceCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", visionURL+"/cam/devices", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Data []cameraDeviceInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	cameraDeviceCacheMu.Lock()
	cameraDeviceCache = result.Data
	cameraDeviceCacheMu.Unlock()
	return nil
}

// ── Geometry helpers ─────────────────────────────────────────────────────────

// positionAtBearingDistance returns the lat/lng reached by travelling
// distanceM metres from (lat,lng) along bearingDeg (0=N, 90=E, …).
func positionAtBearingDistance(lat, lng, bearingDeg, distanceM float64) (float64, float64) {
	const R = 6371000.0 // Earth radius in metres
	δ := distanceM / R
	θ := bearingDeg * math.Pi / 180
	φ1 := lat * math.Pi / 180
	λ1 := lng * math.Pi / 180

	φ2 := math.Asin(math.Sin(φ1)*math.Cos(δ) +
		math.Cos(φ1)*math.Sin(δ)*math.Cos(θ))
	λ2 := λ1 + math.Atan2(
		math.Sin(θ)*math.Sin(δ)*math.Cos(φ1),
		math.Cos(δ)-math.Sin(φ1)*math.Sin(φ2),
	)
	return φ2 * 180 / math.Pi, λ2 * 180 / math.Pi
}

// calcISAPIAzimuth converts a compass bearing to a Hikvision ISAPI azimuth value.
// northOffset is the compass bearing the camera faces when ISAPI azimuth = 0.
// Camera is 360°, so no clamping needed.
// ISAPI azimuth unit: tenths of a degree (0 = 0°, 3600 = 360°).
func calcISAPIAzimuth(compassBearing, northOffset float64) int {
	offset := math.Mod(compassBearing-northOffset+360, 360)
	return int(math.Round(offset * 10))
}

// calcISAPIElevation returns an ISAPI elevation value (tenths of degrees, -900..900)
// for a drone at the given altitude (metres ASL) seen from a camera at cameraH metres,
// with a horizontal distance of distanceM metres.
// Positive elevation = looking up; negative = looking down.
func calcISAPIElevation(droneAltM int, distanceM int) int {
	if distanceM <= 0 {
		return 0
	}
	heightDiff := float64(droneAltM) - cameraHeightM
	angleDeg := math.Atan2(heightDiff, float64(distanceM)) * 180 / math.Pi
	isapi := int(math.Round(angleDeg * 10))
	// Clamp to ISAPI range
	if isapi > 900 {
		isapi = 900
	}
	if isapi < -900 {
		isapi = -900
	}
	return isapi
}

// ── PTZ command ──────────────────────────────────────────────────────────────

func sendCameraPTZ(label string, deviceID, azimuth, elevation int) {
	url := fmt.Sprintf("%s/cam/devices/%d/pantilt", visionURL, deviceID)
	payload := map[string]int{"azimuth": azimuth, "elevation": elevation}
	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[%s] camera_track: failed to build PTZ request for device %d: %v", label, deviceID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] camera_track: PTZ request failed for device %d: %v", label, deviceID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] camera_track: PTZ device %d returned HTTP %d", label, deviceID, resp.StatusCode)
		return
	}
	log.Printf("[%s] camera_track: device %d → azimuth=%d elevation=%d", label, deviceID, azimuth, elevation)
	// Publish camera heading to MQTT so the map can show a live direction stripe.
	if mqttDetectPublisher != nil {
		msg, _ := json.Marshal(map[string]int{"azimuth": azimuth})
		_ = mqttDetectPublisher.Publish(fmt.Sprintf("cam/%d/heading", deviceID), 0, true, string(msg))
	}
}

// ── Main entry point ─────────────────────────────────────────────────────────

// autoTrackCamera computes the bearing from each configured camera to the drone
// and sends an absolute PTZ command to point the camera at it.
// Called from parseDroneData in drone_detector.go.
func autoTrackCamera(label string, d DroneData) {
	// Parse detector name from label "detector-{id}-{name}"
	parts := splitN3(label)
	if parts == "" {
		return
	}
	detectorName := parts

	deviceIDs, ok := cameraTrackRules[detectorName]
	if !ok {
		return // no cameras configured for this detector
	}

	// Determine the drone's effective position.
	droneLat := float64(d.DroneLatitude)
	droneLng := float64(d.DroneLongitude)
	hasDroneGPS := !(droneLat == 0 && droneLng == 0)

	detectorLocationsMu.RLock()
	detLoc, hasDetLoc := detectorLocations[label]
	detectorLocationsMu.RUnlock()

	for _, devID := range deviceIDs {
		cam, ok := getCameraDevice(devID)
		if !ok {
			continue
		}

		var compassBearing float64
		var horizontalDist int

		if hasDroneGPS {
			// Dynamic: compute bearing from camera to actual drone position.
			compassBearing = calcBearing(cam.Lat, cam.Lng, droneLat, droneLng)
			// Horizontal distance from camera to drone (ignoring altitude).
			horizontalDist = int(haversineM(cam.Lat, cam.Lng, droneLat, droneLng))
			log.Printf("[%s] camera_track: device %d — drone GPS known, bearing=%.1f° dist=%dm",
				label, devID, compassBearing, horizontalDist)
		} else if hasDetLoc {
			// Fallback: project a virtual drone position from the detector
			// using the detection heading and the configured range constant.
			virtualLat, virtualLng := positionAtBearingDistance(
				detLoc[0], detLoc[1],
				float64(d.DirectionAngle),
				droneRangeFallbackM,
			)
			compassBearing = calcBearing(cam.Lat, cam.Lng, virtualLat, virtualLng)
			horizontalDist = int(haversineM(cam.Lat, cam.Lng, virtualLat, virtualLng))
			log.Printf("[%s] camera_track: device %d — no drone GPS, virtual pos (%.4f,%.4f), bearing=%.1f° dist=%dm",
				label, devID, virtualLat, virtualLng, compassBearing, horizontalDist)
		} else {
			log.Printf("[%s] camera_track: device %d — no drone GPS and detector location unknown, skipping", label, devID)
			continue
		}

		northOff := northOffsets[devID] // 0 if not configured
		azimuth := calcISAPIAzimuth(compassBearing, northOff)
		elevation := calcISAPIElevation(int(d.DroneAltitude), horizontalDist)

		go sendCameraPTZ(label, devID, azimuth, elevation)
		go startCameraRecord(label, devID)
	}
}

// startCameraRecord asks the vision server to record both normal and thermal
// cameras for this device for 20 seconds. If a recording is already in
// progress the vision server returns 409 Conflict and the error is silently
// ignored, so multiple detections do not stack recordings.
func startCameraRecord(label string, deviceID int) {
	const recordSeconds = 20
	for _, cam := range []string{"normal", "thermal"} {
		camName := cam // capture for goroutine
		url := fmt.Sprintf("%s/cam/devices/%d/record/start", visionURL, deviceID)
		payload := map[string]any{"cam": camName, "duration": recordSeconds, "detect": camName == "normal"}
		body, _ := json.Marshal(payload)

		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
			if err != nil {
				log.Printf("[%s] camera_track: failed to build record request for device %d %s: %v", label, deviceID, camName, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[%s] camera_track: record request failed for device %d %s: %v", label, deviceID, camName, err)
				return
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				log.Printf("[%s] camera_track: device %d %s recording started (%ds)", label, deviceID, camName, recordSeconds)
			case http.StatusConflict:
				// Already recording — no action needed.
			default:
				log.Printf("[%s] camera_track: device %d %s record returned HTTP %d", label, deviceID, camName, resp.StatusCode)
			}
		}()
	}
}

// splitN3 extracts the third segment of a "a-b-c" label, or returns "".
func splitN3(label string) string {
	first := -1
	for i, c := range label {
		if c == '-' {
			if first == -1 {
				first = i
			} else {
				return label[i+1:]
			}
		}
	}
	return ""
}

// haversineM returns the great-circle distance in metres between two lat/lng points.
func haversineM(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
