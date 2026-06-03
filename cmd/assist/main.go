package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	internalmqtt "dblocker_control/internal/infrastructure/mqtt"
)

type DBlockerConfig struct {
	SignalGPS  bool `json:"signal_gps"`
	SignalCtrl bool `json:"signal_ctrl"`
}

type DBlocker struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	SerialNumb string `json:"serial_numb"`
}

type Schedule struct {
	ID         uint             `json:"id"`
	DBlockerID uint             `json:"dblocker_id"`
	DBlocker   DBlocker         `json:"dblocker"`
	Config     []DBlockerConfig `json:"config"`
	Time       string           `json:"time"` // UTC HH:MM
	CreatedBy  string           `json:"created_by"`
	Enabled    bool             `json:"enabled"`
}

type ScheduleResponse struct {
	Data []Schedule `json:"data"`
}

type ConfigUpdatePayload struct {
	ID     uint              `json:"id"`
	Config [6]DBlockerConfig `json:"config"`
}

var (
	backendURL string
	apiKey     string
	client     = &http.Client{Timeout: 10 * time.Second}
)

func main() {
	backendURL = os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://dblocker-app:8080"
	}
	apiKey = os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY is required")
	}

	// Camera tracking setup
	visionURL = os.Getenv("VISION_URL")
	if visionURL == "" {
		visionURL = "http://dblocker-vision:8090"
	}
	if v := os.Getenv("DRONE_RANGE_FALLBACK"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			droneRangeFallbackM = f
		}
	}
	if v := os.Getenv("CAMERA_HEIGHT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cameraHeightM = f
		}
	}
	for i := 1; i <= 8; i++ {
		key := fmt.Sprintf("DEVICE_%d_NORTH_OFFSET", i)
		if v := os.Getenv(key); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				northOffsets[i] = f
			}
		}
	}
	log.Printf("camera tracking: vision=%s range_fallback=%.0fm camera_height=%.0fm",
		visionURL, droneRangeFallbackM, cameraHeightM)

	log.Println("dblocker-assist started, polling schedules...")

	// Connect to MQTT for drone detection live feed publishing.
	mqttBroker := os.Getenv("MQTT_BROKER")
	if mqttBroker == "" {
		mqttBroker = "tcp://mosquitto:1883"
	}
	mqttUser := os.Getenv("MQTT_USERNAME")
	mqttPass := os.Getenv("MQTT_PASSWORD")
	if mqc, err := internalmqtt.NewWithAuth(mqttBroker, "dblocker-assist-detector", mqttUser, mqttPass); err != nil {
		log.Printf("warn: MQTT connect failed, drone detection live feed disabled: %v", err)
	} else {
		mqttDetectPublisher = mqc
		log.Printf("MQTT connected for drone detection live feed (%s)", mqttBroker)
	}

	// Fetch settings synchronously first so the initial state of
	// auto_blocker/auto_camera is correct before any detection arrives.
	fetchAndUpdateHoldSeconds()

	// Start drone detector connections from database
	startDetectorsFromDB()
	startHoldSettingSync()
	startDetectionCacheCleanup()
	startPendingRestoreRetry()

	// Track which schedules already executed this minute to avoid duplicates.
	// Key: "scheduleID:HH:MM"
	executed := map[string]bool{}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Run immediately on start, then every tick
	runSchedules(executed)
	for range ticker.C {
		runSchedules(executed)
	}
}

func runSchedules(executed map[string]bool) {
	nowUTC := time.Now().UTC()
	currentTime := fmt.Sprintf("%02d:%02d", nowUTC.Hour(), nowUTC.Minute())

	// Clean old entries (anything not matching current minute)
	for k := range executed {
		// Keys are "id:HH:MM" — safety check length before slicing
		if len(k) >= 5 {
			parts := k[len(k)-5:]
			if parts != currentTime {
				delete(executed, k)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schedules, err := fetchSchedules(ctx)
	if err != nil {
		log.Printf("error fetching schedules: %v", err)
		return
	}

	for _, s := range schedules {
		if !s.Enabled {
			continue
		}
		if s.Time != currentTime {
			continue
		}

		key := fmt.Sprintf("%d:%s", s.ID, currentTime)
		if executed[key] {
			continue
		}

		log.Printf("executing schedule #%d for dblocker #%d at %s UTC", s.ID, s.DBlockerID, currentTime)
		if err := applyConfig(ctx, s); err != nil {
			log.Printf("error applying schedule #%d: %v", s.ID, err)
			continue
		}
		executed[key] = true
		log.Printf("schedule #%d applied successfully", s.ID)

		// Log the action
		if err := createActionLog(ctx, s); err != nil {
			log.Printf("warn: failed to log schedule #%d: %v", s.ID, err)
		}
	}
}

func fetchSchedules(ctx context.Context) ([]Schedule, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", backendURL+"/api/schedules", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func applyConfig(ctx context.Context, s Schedule) error {
	var config [6]DBlockerConfig
	for i := 0; i < 6 && i < len(s.Config); i++ {
		config[i] = s.Config[i]
	}

	payload := ConfigUpdatePayload{
		ID:     s.DBlockerID,
		Config: config,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", backendURL+"/api/dblockers/config", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type ActionLogPayload struct {
	Username     string           `json:"username"`
	Action       string           `json:"action"`
	DBlockerID   uint             `json:"dblocker_id"`
	DBlockerName string           `json:"dblocker_name"`
	Config       []DBlockerConfig `json:"config"`
}

func createActionLog(ctx context.Context, s Schedule) error {
	payload := ActionLogPayload{
		Username:     fmt.Sprintf("assistant[%s]", s.CreatedBy),
		Action:       "scheduled_config_update",
		DBlockerID:   s.DBlockerID,
		DBlockerName: s.DBlocker.Name,
		Config:       s.Config,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", backendURL+"/api/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type DetectorEntry struct {
	ID   uint    `json:"id"`
	Name string  `json:"name"`
	Host string  `json:"host"`
	Port int     `json:"port"`
	Lat  float64 `json:"latitude"`
	Lng  float64 `json:"longitude"`
}

// startedDetectors tracks which detector IDs have already had a goroutine launched,
// so the polling loop never starts a second connection for the same detector.
var (
	startedDetectorsMu sync.Mutex
	startedDetectors   = map[uint]bool{}
)

func startDetectorsFromDB() {
	go func() {
		for {
			fetchAndStartNewDetectors()
			time.Sleep(30 * time.Second)
		}
	}()
}

func fetchAndStartNewDetectors() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", backendURL+"/api/detectors", nil)
	if err != nil {
		log.Printf("warn: failed to create detector request: %v", err)
		return
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("warn: failed to fetch detectors from DB: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("warn: detector fetch returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Data []DetectorEntry `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("warn: failed to decode detectors: %v", err)
		return
	}

	startedDetectorsMu.Lock()
	defer startedDetectorsMu.Unlock()

	for _, d := range result.Data {
		if startedDetectors[d.ID] {
			continue // already running
		}
		startedDetectors[d.ID] = true
		label := fmt.Sprintf("detector-%d-%s", d.ID, d.Name)
		log.Printf("starting detector %q at %s:%d (%.6f, %.6f)", label, d.Host, d.Port, d.Lat, d.Lng)
		go StartDroneDetector(label, d.Host, d.Port, d.Lat, d.Lng)
	}
}
