package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"dblocker_control/internal/service"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DBlockerHandler struct {
	Repo       *repository.DBlockerRepository
	MqttClient mqtt.Client
}

func NewDBlockerHandler(repo *repository.DBlockerRepository, mqttClient mqtt.Client) *DBlockerHandler {
	return &DBlockerHandler{Repo: repo, MqttClient: mqttClient}
}

func (h *DBlockerHandler) CreateDBlocker(c *gin.Context) {
	var input models.DBlocker
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If no config provided, initialize 6 sectors with defaults (all false).
	if len(input.Config) == 0 {
		input.Config = make([]models.DBlockerConfig, 6)
	}

	if err := h.Repo.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": input})
}

func (h *DBlockerHandler) GetDBlockers(c *gin.Context) {
	dblockers, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dblockers})
}

func (h *DBlockerHandler) GetDBlockerByID(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	dblocker, err := h.Repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DBlocker not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dblocker})
}

func (h *DBlockerHandler) UpdateDBlocker(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}
	var input models.DBlocker
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.ID = uint(id)

	if err := h.Repo.Update(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": input})
}

func (h *DBlockerHandler) DeleteDBlocker(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.Repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DBlocker deleted successfully"})
}

func (h *DBlockerHandler) UpdateDBlockerConfig(c *gin.Context) {
	var input models.DBlockerConfigUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dblocker, err := h.Repo.FindByID(input.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dblocker not found"})
		return
	}

	if err := h.Repo.UpdateConfig(input.ID, input.Config[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	topic := fmt.Sprintf("dbl/%s/cmd", dblocker.SerialNumb)

	// payload, err := json.Marshal(input.Config)

	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		input.Config[:],
		true,
		true,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	payload := []byte{
		byte(bitmaskPayload >> 8),
		byte(bitmaskPayload),
	}

	if err := h.MqttClient.Publish(topic, 1, true, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish to mqtt"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": input})
}
