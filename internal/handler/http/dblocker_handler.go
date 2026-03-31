package http

import (
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"dblocker_control/internal/service"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DBlockerHandler struct {
	Repo       *repository.DBlockerRepository
	LogRepo    *repository.ActionLogRepository
	MqttClient mqtt.Client
	Bridge     *service.BridgeService
}

func NewDBlockerHandler(repo *repository.DBlockerRepository, logRepo *repository.ActionLogRepository, mqttClient mqtt.Client, bridge *service.BridgeService) *DBlockerHandler {
	return &DBlockerHandler{Repo: repo, LogRepo: logRepo, MqttClient: mqttClient, Bridge: bridge}
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

	if h.Bridge != nil {
		if err := h.Bridge.RefreshTopics(); err != nil {
			// DB write succeeded; do not fail the request. Topics will resync on the
			// next CRUD operation or app restart.
			log.Printf("warn: RefreshTopics after create dblocker %d: %v", input.ID, err)
		}
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

	if h.Bridge != nil {
		if err := h.Bridge.RefreshTopics(); err != nil {
			log.Printf("warn: RefreshTopics after update dblocker %d: %v", input.ID, err)
		}
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

	if h.Bridge != nil {
		if err := h.Bridge.RefreshTopics(); err != nil {
			log.Printf("warn: RefreshTopics after delete dblocker %d: %v", id, err)
		}
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

	// Snapshot current readings before sending the new config
	if h.Bridge != nil && h.Bridge.Monitor() != nil {
		rptPayload := h.Bridge.LastRpt(dblocker.SerialNumb)
		if rptPayload != "" {
			cfgSlice := make([]service.SectorConfig, 6)
			for i := 0; i < 6; i++ {
				cfgSlice[i] = service.SectorConfig{
					Ctrl: input.Config[i].SignalCtrl,
					GPS:  input.Config[i].SignalGPS,
				}
			}
			h.Bridge.Monitor().Snapshot(dblocker.SerialNumb, rptPayload, cfgSlice)
		}
	}

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

	// Log action — skip for _service (assist app logs its own "scheduled_config_update")
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil && username != "_service" {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "config_update",
			DBlockerID:   input.ID,
			DBlockerName: dblocker.Name,
			Config:       input.Config[:],
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": input})
}

func (h *DBlockerHandler) TurnOffAllDBlockerConfig(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	dblocker, err := h.Repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dblocker not found"})
		return
	}

	// Create config with all values set to false (turned off)
	allOffConfig := [6]models.DBlockerConfig{
		{SignalGPS: false, SignalCtrl: false},
		{SignalGPS: false, SignalCtrl: false},
		{SignalGPS: false, SignalCtrl: false},
		{SignalGPS: false, SignalCtrl: false},
		{SignalGPS: false, SignalCtrl: false},
		{SignalGPS: false, SignalCtrl: false},
	}

	if err := h.Repo.UpdateConfig(uint(id), allOffConfig[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	topic := fmt.Sprintf("dbl/%s/cmd", dblocker.SerialNumb)

	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		allOffConfig[:],
		false,
		false,
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

	c.JSON(http.StatusOK, gin.H{
		"message": "All DBlocker config turned off successfully",
		"data": gin.H{
			"id":     uint(id),
			"config": allOffConfig,
		},
	})

	// Log action
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "turn_off_all",
			DBlockerID:   uint(id),
			DBlockerName: dblocker.Name,
			Config:       allOffConfig[:],
		})
	}
}

// GetMonitorStatus returns the current monitor errors for all dblockers.
func (h *DBlockerHandler) GetMonitorStatus(c *gin.Context) {
	if h.Bridge == nil || h.Bridge.Monitor() == nil {
		c.JSON(http.StatusOK, gin.H{"data": map[string]any{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.Bridge.Monitor().StatusAll()})
}

func (h *DBlockerHandler) PresetOnDBlockerConfig(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	dblocker, err := h.Repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dblocker not found"})
		return
	}

	if len(dblocker.PresetConfig) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "preset config not set for this dblocker"})
		return
	}

	if err := h.Repo.UpdateConfig(uint(id), dblocker.PresetConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	topic := fmt.Sprintf("dbl/%s/cmd", dblocker.SerialNumb)

	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		dblocker.PresetConfig,
		true,
		true,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Preset ON config applied successfully",
		"data": gin.H{
			"id":     uint(id),
			"config": dblocker.PresetConfig,
		},
	})

	// Log action
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "preset_on",
			DBlockerID:   uint(id),
			DBlockerName: dblocker.Name,
			Config:       dblocker.PresetConfig,
		})
	}
}

func (h *DBlockerHandler) UpdatePresetConfig(c *gin.Context) {
	var input models.DBlockerConfigUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.Repo.FindByID(input.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dblocker not found"})
		return
	}

	if err := h.Repo.UpdatePresetConfig(input.ID, input.Config[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Preset config saved successfully",
		"data":    input,
	})
}
