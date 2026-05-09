package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// detectionHoldSecs is the number of seconds a dblocker stays ON after the last detection.
// Default is 30. Guarded by detectionHoldSecsMu.
var (
	detectionHoldSecs   int = 30
	detectionHoldSecsMu sync.RWMutex

	detectionAutoBlocker   bool = true
	detectionAutoCamera    bool = true
	detectionAutoMu        sync.RWMutex
)

type DetectorHandler struct {
	DetectorRepo   *repository.DetectorRepository
	EventRepo      *repository.DroneEventRepository
	SettingRepo    *repository.AppSettingRepository
}

func NewDetectorHandler(detectorRepo *repository.DetectorRepository, eventRepo *repository.DroneEventRepository) *DetectorHandler {
	return &DetectorHandler{DetectorRepo: detectorRepo, EventRepo: eventRepo}
}

func NewDetectorHandlerWithSettings(detectorRepo *repository.DetectorRepository, eventRepo *repository.DroneEventRepository, settingRepo *repository.AppSettingRepository) *DetectorHandler {
	h := &DetectorHandler{DetectorRepo: detectorRepo, EventRepo: eventRepo, SettingRepo: settingRepo}
	h.loadSettingsFromDB()
	return h
}

// loadSettingsFromDB reads persisted settings at startup.
func (h *DetectorHandler) loadSettingsFromDB() {
	if h.SettingRepo == nil {
		return
	}
	if v := h.SettingRepo.Get("detection.hold_seconds", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 && n <= 3600 {
			detectionHoldSecsMu.Lock()
			detectionHoldSecs = n
			detectionHoldSecsMu.Unlock()
		}
	}
	detectionAutoMu.Lock()
	detectionAutoBlocker = h.SettingRepo.Get("detection.auto_blocker", "true") != "false"
	detectionAutoCamera = h.SettingRepo.Get("detection.auto_camera", "true") != "false"
	detectionAutoMu.Unlock()
}

// --- Detector CRUD ---

func (h *DetectorHandler) CreateDetector(c *gin.Context) {
	var input models.DroneDetector
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DetectorRepo.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": input})
}

func (h *DetectorHandler) GetDetectors(c *gin.Context) {
	detectors, err := h.DetectorRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detectors})
}

func (h *DetectorHandler) UpdateDetector(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var input models.DroneDetector
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.ID = uint(id)
	if err := h.DetectorRepo.Update(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": input})
}

func (h *DetectorHandler) DeleteDetector(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.DetectorRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Detector deleted"})
}

// --- Drone Events ---

func (h *DetectorHandler) GetDroneEvents(c *gin.Context) {
	now := time.Now().UTC()

	fromStr := c.DefaultQuery("from", now.Format("2006-01-02"))
	toStr := c.DefaultQuery("to", now.Format("2006-01-02"))

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date, use YYYY-MM-DD"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date, use YYYY-MM-DD"})
		return
	}
	to = to.Add(24*time.Hour - time.Nanosecond)

	limitStr := c.DefaultQuery("limit", "200")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	var detectorID uint
	if idStr := c.Query("detector_id"); idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid detector_id"})
			return
		}
		detectorID = uint(id)
	}

	events, err := h.EventRepo.FindFiltered(from, to, detectorID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}

func (h *DetectorHandler) DeleteDroneEventsByDate(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both 'from' and 'to' query params required (YYYY-MM-DD)"})
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date"})
		return
	}
	to = to.Add(24*time.Hour - time.Nanosecond)

	deleted, err := h.EventRepo.DeleteByDateRange(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "drone events deleted", "deleted": deleted})
}

func (h *DetectorHandler) CreateDroneEvent(c *gin.Context) {
	var input models.DroneEvent
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.EventRepo.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": input})
}

func (h *DetectorHandler) UpdateDetectorStatus(c *gin.Context) {
	var input struct {
		Host   string `json:"host" binding:"required"`
		Port   int    `json:"port" binding:"required"`
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status != "online" && input.Status != "offline" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'online' or 'offline'"})
		return
	}

	detector, err := h.DetectorRepo.FindByHostPort(input.Host, input.Port)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "detector not found"})
		return
	}

	if err := h.DetectorRepo.UpdateStatus(detector.ID, input.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "status updated", "detector_id": detector.ID, "status": input.Status})
}

// GetDetectionSettings returns the current detection settings.
func (h *DetectorHandler) GetDetectionSettings(c *gin.Context) {
	detectionHoldSecsMu.RLock()
	secs := detectionHoldSecs
	detectionHoldSecsMu.RUnlock()
	detectionAutoMu.RLock()
	autoBlocker := detectionAutoBlocker
	autoCamera := detectionAutoCamera
	detectionAutoMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"hold_seconds":  secs,
		"auto_blocker":  autoBlocker,
		"auto_camera":   autoCamera,
	}})
}

// UpdateDetectionSettings updates detection settings and persists them to the database.
func (h *DetectorHandler) UpdateDetectionSettings(c *gin.Context) {
	var input struct {
		HoldSeconds *int  `json:"hold_seconds"`
		AutoBlocker *bool `json:"auto_blocker"`
		AutoCamera  *bool `json:"auto_camera"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.HoldSeconds != nil {
		if *input.HoldSeconds < 5 || *input.HoldSeconds > 3600 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hold_seconds must be between 5 and 3600"})
			return
		}
		detectionHoldSecsMu.Lock()
		detectionHoldSecs = *input.HoldSeconds
		detectionHoldSecsMu.Unlock()
		if h.SettingRepo != nil {
			_ = h.SettingRepo.Set("detection.hold_seconds", strconv.Itoa(*input.HoldSeconds))
		}
	}
	if input.AutoBlocker != nil {
		detectionAutoMu.Lock()
		detectionAutoBlocker = *input.AutoBlocker
		detectionAutoMu.Unlock()
		if h.SettingRepo != nil {
			v := "false"
			if *input.AutoBlocker {
				v = "true"
			}
			_ = h.SettingRepo.Set("detection.auto_blocker", v)
		}
	}
	if input.AutoCamera != nil {
		detectionAutoMu.Lock()
		detectionAutoCamera = *input.AutoCamera
		detectionAutoMu.Unlock()
		if h.SettingRepo != nil {
			v := "false"
			if *input.AutoCamera {
				v = "true"
			}
			_ = h.SettingRepo.Set("detection.auto_camera", v)
		}
	}
	detectionHoldSecsMu.RLock()
	secs := detectionHoldSecs
	detectionHoldSecsMu.RUnlock()
	detectionAutoMu.RLock()
	autoBlocker := detectionAutoBlocker
	autoCamera := detectionAutoCamera
	detectionAutoMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"hold_seconds":  secs,
		"auto_blocker":  autoBlocker,
		"auto_camera":   autoCamera,
	}})
}
