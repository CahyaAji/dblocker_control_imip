package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type DetectorHandler struct {
	DetectorRepo *repository.DetectorRepository
	EventRepo    *repository.DroneEventRepository
}

func NewDetectorHandler(detectorRepo *repository.DetectorRepository, eventRepo *repository.DroneEventRepository) *DetectorHandler {
	return &DetectorHandler{DetectorRepo: detectorRepo, EventRepo: eventRepo}
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

	// Dedup: skip if latest event for same target has heading within 5°
	if input.TargetName != "" {
		if last, err := h.EventRepo.FindLatestByTarget(input.TargetName); err == nil {
			diff := math.Abs(float64(input.Heading - last.Heading))
			if diff > 180 {
				diff = 360 - diff
			}
			if diff < 5 {
				c.JSON(http.StatusOK, gin.H{"data": last, "deduplicated": true})
				return
			}
		}
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
