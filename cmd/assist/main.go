package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type DBlockerConfig struct {
	SignalGPS  bool `json:"signal_gps"`
	SignalCtrl bool `json:"signal_ctrl"`
}

type Schedule struct {
	ID         uint             `json:"id"`
	DBlockerID uint             `json:"dblocker_id"`
	Config     []DBlockerConfig `json:"config"`
	Time       string           `json:"time"` // UTC HH:MM
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

	log.Println("dblocker-assist started, polling schedules...")

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
		// Keys are "id:HH:MM" — keep only current minute
		if len(k) > 0 {
			parts := k[len(k)-5:]
			if parts != currentTime {
				delete(executed, k)
			}
		}
	}

	schedules, err := fetchSchedules()
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
		if err := applyConfig(s); err != nil {
			log.Printf("error applying schedule #%d: %v", s.ID, err)
			continue
		}
		executed[key] = true
		log.Printf("schedule #%d applied successfully", s.ID)
	}
}

func fetchSchedules() ([]Schedule, error) {
	req, err := http.NewRequest("GET", backendURL+"/api/schedules", nil)
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

func applyConfig(s Schedule) error {
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

	req, err := http.NewRequest("PUT", backendURL+"/api/dblockers/config", bytes.NewReader(body))
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
