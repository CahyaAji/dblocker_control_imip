package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ActionLogHandler struct {
	Repo *repository.ActionLogRepository
}

func NewActionLogHandler(repo *repository.ActionLogRepository) *ActionLogHandler {
	return &ActionLogHandler{Repo: repo}
}

func (h *ActionLogHandler) GetLogs(c *gin.Context) {
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
	// Set 'to' to end of day
	to = to.Add(24*time.Hour - time.Nanosecond)

	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'from' must be before 'to'"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 500 {
		limit = 50
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	logs, total, err := h.Repo.FindByDateRange(from, to, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": total,
	})
}

func (h *ActionLogHandler) DeleteLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	yearStr := c.Query("year")
	if yearStr == "" {
		yearStr = strconv.Itoa(time.Now().UTC().Year())
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
		return
	}

	if err := h.Repo.Delete(uint(id), year); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "log deleted"})
}

func (h *ActionLogHandler) CreateLog(c *gin.Context) {
	var input models.ActionLog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use server time
	input.Timestamp = time.Now().UTC()

	// Identify caller
	user, _ := c.Get("user")
	if u, ok := user.(*models.User); ok {
		// Allow service account to pass custom username (e.g. "assistant[admin]")
		if u.Username == "_service" && input.Username != "" {
			// keep input.Username as-is
		} else {
			input.Username = u.Username
		}
	}

	if err := h.Repo.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": input})
}
