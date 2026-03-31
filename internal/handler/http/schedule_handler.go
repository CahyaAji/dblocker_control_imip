package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ScheduleHandler struct {
	Repo *repository.ScheduleRepository
}

func NewScheduleHandler(repo *repository.ScheduleRepository) *ScheduleHandler {
	return &ScheduleHandler{Repo: repo}
}

var timeRegex = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
var tzRegex = regexp.MustCompile(`^[+-](0\d|1[0-4]):(00|15|30|45)$`)

// localToUTC converts a local HH:MM + timezone offset (e.g. "+08:00") to UTC HH:MM.
func localToUTC(localTime string, tz string) (string, error) {
	var h, m int
	if _, err := fmt.Sscanf(localTime, "%d:%d", &h, &m); err != nil {
		return "", err
	}

	sign := 1
	if tz[0] == '-' {
		sign = -1
	}
	var tzH, tzM int
	if _, err := fmt.Sscanf(tz[1:], "%d:%d", &tzH, &tzM); err != nil {
		return "", err
	}
	offsetMinutes := sign * (tzH*60 + tzM)

	totalMinutes := h*60 + m - offsetMinutes
	// Wrap around 24h
	totalMinutes = ((totalMinutes % 1440) + 1440) % 1440

	return fmt.Sprintf("%02d:%02d", totalMinutes/60, totalMinutes%60), nil
}

func (h *ScheduleHandler) CreateSchedule(c *gin.Context) {
	var input models.CreateScheduleRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !timeRegex.MatchString(input.Time) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "time must be in HH:MM format (00:00–23:59)"})
		return
	}

	if !tzRegex.MatchString(input.Timezone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "timezone must be in ±HH:MM format (e.g. +08:00)"})
		return
	}

	utcTime, err := localToUTC(input.Time, input.Timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to convert time to UTC"})
		return
	}

	schedule := models.Schedule{
		DBlockerID: input.DBlockerID,
		Config:     input.Config,
		Time:       utcTime,
		Timezone:   input.Timezone,
		Enabled:    true,
	}

	if err := h.Repo.Create(&schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": schedule})
}

func (h *ScheduleHandler) GetSchedules(c *gin.Context) {
	schedules, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": schedules})
}

func (h *ScheduleHandler) ToggleSchedule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	schedule, err := h.Repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}

	schedule.Enabled = !schedule.Enabled
	if err := h.Repo.Update(schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": schedule})
}

func (h *ScheduleHandler) DeleteSchedule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.Repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "schedule deleted"})
}
