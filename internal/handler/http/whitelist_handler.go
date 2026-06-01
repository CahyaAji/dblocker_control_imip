package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WhitelistHandler struct {
	Repo *repository.WhitelistRepository
}

func NewWhitelistHandler(repo *repository.WhitelistRepository) *WhitelistHandler {
	return &WhitelistHandler{Repo: repo}
}

func (h *WhitelistHandler) GetWhitelist(c *gin.Context) {
	entries, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *WhitelistHandler) CreateWhitelistEntry(c *gin.Context) {
	var input models.DroneWhitelist
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": input})
}

func (h *WhitelistHandler) DeleteWhitelistEntry(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.Repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Whitelist entry deleted"})
}
