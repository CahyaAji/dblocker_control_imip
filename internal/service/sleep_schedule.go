package service

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SleepScheduleService sends global SLEEP/WAKE commands to all dblockers at configured times.
type SleepScheduleService struct {
	mu         sync.RWMutex
	enabled    bool
	sleepTime  string // "HH:MM"
	wakeTime   string // "HH:MM"
	timezone   string // e.g. "+07:00"
	repo       *repository.DBlockerRepository
	mqttClient mqtt.Client
	fanCtrl    *FanControlService
	stopCh     chan struct{}
}

func NewSleepScheduleService(repo *repository.DBlockerRepository, mqttClient mqtt.Client, fanCtrl *FanControlService) *SleepScheduleService {
	s := &SleepScheduleService{
		enabled:    false,
		sleepTime:  "22:00",
		wakeTime:   "06:00",
		timezone:   "+07:00",
		repo:       repo,
		mqttClient: mqttClient,
		fanCtrl:    fanCtrl,
		stopCh:     make(chan struct{}),
	}
	go s.run()
	return s
}

// GetSettings returns the current sleep schedule settings.
func (s *SleepScheduleService) GetSettings() (enabled bool, sleepTime, wakeTime, timezone string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled, s.sleepTime, s.wakeTime, s.timezone
}

// SetSettings updates the sleep schedule settings.
func (s *SleepScheduleService) SetSettings(enabled bool, sleepTime, wakeTime, timezone string) error {
	if err := validateHHMM(sleepTime); err != nil {
		return fmt.Errorf("invalid sleep_time: %w", err)
	}
	if err := validateHHMM(wakeTime); err != nil {
		return fmt.Errorf("invalid wake_time: %w", err)
	}
	if _, err := parseTimezone(timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	s.sleepTime = sleepTime
	s.wakeTime = wakeTime
	s.timezone = timezone
	return nil
}

// run is the background goroutine that checks the schedule every minute.
func (s *SleepScheduleService) run() {
	// Align to the next full minute
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.check()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SleepScheduleService) check() {
	s.mu.RLock()
	enabled := s.enabled
	sleepTime := s.sleepTime
	wakeTime := s.wakeTime
	tz := s.timezone
	s.mu.RUnlock()

	if !enabled {
		return
	}

	offset, err := parseTimezone(tz)
	if err != nil {
		return
	}

	loc := time.FixedZone("configured", int(offset.Seconds()))
	now := time.Now().In(loc)
	current := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	if current == sleepTime {
		log.Printf("sleep_schedule: triggering SLEEP for all dblockers at %s (%s)", current, tz)
		s.sendToAll("sleep")
	} else if current == wakeTime {
		log.Printf("sleep_schedule: triggering WAKE for all dblockers at %s (%s)", current, tz)
		s.sendToAll("wake")
	}
}

func (s *SleepScheduleService) sendToAll(action string) {
	dblockers, err := s.repo.FindAll()
	if err != nil {
		log.Printf("sleep_schedule: failed to load dblockers: %v", err)
		return
	}

	for _, d := range dblockers {
		serial := d.SerialNumb
		topic := fmt.Sprintf("dbl/%s/cmd", serial)

		if action == "sleep" {
			// Turn off all sectors then send SLEEP
			allOff := make([]models.DBlockerConfig, 6)
			if err := s.repo.UpdateConfig(d.ID, allOff); err != nil {
				log.Printf("sleep_schedule: failed to update config for %s: %v", serial, err)
				continue
			}
			var fanM, fanS bool
			if s.fanCtrl != nil {
				fanM, fanS = s.fanCtrl.FanState(serial, allOff)
			}
			bitmask, err := DBlockerConfigToBitmask(allOff, fanM, fanS)
			if err != nil {
				log.Printf("sleep_schedule: failed to build bitmask for %s: %v", serial, err)
				continue
			}
			offPayload := []byte{byte(bitmask >> 8), byte(bitmask)}
			if err := s.mqttClient.Publish(topic, 1, true, offPayload); err != nil {
				log.Printf("sleep_schedule: failed to publish off cmd for %s: %v", serial, err)
				continue
			}
			if err := s.mqttClient.Publish(topic, 1, false, []byte("SLEEP")); err != nil {
				log.Printf("sleep_schedule: failed to publish SLEEP for %s: %v", serial, err)
			}
		} else {
			if err := s.mqttClient.Publish(topic, 1, false, []byte("WAKE")); err != nil {
				log.Printf("sleep_schedule: failed to publish WAKE for %s: %v", serial, err)
			}
		}
	}
}

// --- helpers ---

func validateHHMM(t string) error {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return fmt.Errorf("must be HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return fmt.Errorf("hour out of range")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return fmt.Errorf("minute out of range")
	}
	return nil
}

func parseTimezone(tz string) (time.Duration, error) {
	if len(tz) != 6 || (tz[0] != '+' && tz[0] != '-') {
		return 0, fmt.Errorf("must be ±HH:MM")
	}
	parts := strings.Split(tz[1:], ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("must be ±HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	total := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
	if tz[0] == '-' {
		total = -total
	}
	return total, nil
}
