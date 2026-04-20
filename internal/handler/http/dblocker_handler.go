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

// fanState returns the automatic fan state for a device given its config.
func (h *DBlockerHandler) fanState(serial string, cfg []models.DBlockerConfig) (bool, bool) {
	if h.Bridge != nil && h.Bridge.FanControl() != nil {
		return h.Bridge.FanControl().FanState(serial, cfg)
	}
	// Fallback: fans follow sectors
	for _, c := range cfg {
		if c.SignalGPS || c.SignalCtrl {
			return true, true
		}
	}
	return false, false
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

	// Fetch old data
	old, err := h.Repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DBlocker not found"})
		return
	}

	// Use a patch struct with pointer fields to distinguish omitted vs zero values
	type patchDBlocker struct {
		Name         *string                  `json:"name"`
		SerialNumb   *string                  `json:"serial_numb"`
		IP           *string                  `json:"ip"`
		Latitude     *float64                 `json:"latitude"`
		Longitude    *float64                 `json:"longitude"`
		Desc         *string                  `json:"desc"`
		AngleStart   *int                     `json:"angle_start"`
		Config       *[]models.DBlockerConfig `json:"config"`
		PresetConfig *[]models.DBlockerConfig `json:"preset_config"`
	}
	var patch patchDBlocker
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := *old // start with old data
	input.ID = uint(id)
	if patch.Name != nil {
		input.Name = *patch.Name
	}
	if patch.SerialNumb != nil {
		input.SerialNumb = *patch.SerialNumb
	}
	if patch.IP != nil {
		input.IP = *patch.IP
	}
	if patch.Latitude != nil {
		input.Lat = *patch.Latitude
	}
	if patch.Longitude != nil {
		input.Lng = *patch.Longitude
	}
	if patch.Desc != nil {
		input.Desc = *patch.Desc
	}
	if patch.AngleStart != nil {
		input.AngleStart = *patch.AngleStart
	}
	if patch.Config != nil {
		input.Config = *patch.Config
	}
	if patch.PresetConfig != nil {
		input.PresetConfig = *patch.PresetConfig
	}

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

	// Block turning on sectors if device is overheating
	if h.Bridge != nil && h.Bridge.FanControl() != nil && h.Bridge.FanControl().IsOverheating(dblocker.SerialNumb) {
		for _, cfg := range input.Config {
			if cfg.SignalGPS || cfg.SignalCtrl {
				c.JSON(http.StatusForbidden, gin.H{"error": "device temperature exceeds safe limit, cannot turn on sectors"})
				return
			}
		}
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

	fanM, fanS := h.fanState(dblocker.SerialNumb, input.Config[:])
	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		input.Config[:],
		fanM,
		fanS,
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

	fanM, fanS := h.fanState(dblocker.SerialNumb, allOffConfig[:])
	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		allOffConfig[:],
		fanM,
		fanS,
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

// GetFanThresholds returns the current fan ON/OFF temperature thresholds.
func (h *DBlockerHandler) GetFanThresholds(c *gin.Context) {
	if h.Bridge == nil || h.Bridge.FanControl() == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"fan_on_temp": 45.0, "fan_off_temp": 35.0}})
		return
	}

	onTemp, offTemp := h.Bridge.FanControl().GetThresholds()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"fan_on_temp": onTemp, "fan_off_temp": offTemp}})
}

// UpdateFanThresholds updates the fan ON/OFF temperature thresholds.
func (h *DBlockerHandler) UpdateFanThresholds(c *gin.Context) {
	var input struct {
		FanOnTemp  float64 `json:"fan_on_temp" binding:"required"`
		FanOffTemp float64 `json:"fan_off_temp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Bridge == nil || h.Bridge.FanControl() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fan control not available"})
		return
	}

	if err := h.Bridge.FanControl().SetThresholds(input.FanOnTemp, input.FanOffTemp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"fan_on_temp": input.FanOnTemp, "fan_off_temp": input.FanOffTemp}})
}

// GetTempLimits returns the current warning and auto-off temperature limits.
func (h *DBlockerHandler) GetTempLimits(c *gin.Context) {
	if h.Bridge == nil || h.Bridge.FanControl() == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"temp_warn_limit": 55.0, "temp_off_limit": 65.0}})
		return
	}

	warnLimit, offLimit := h.Bridge.FanControl().GetTempLimits()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"temp_warn_limit": warnLimit, "temp_off_limit": offLimit}})
}

// UpdateTempLimits updates the warning and auto-off temperature limits.
func (h *DBlockerHandler) UpdateTempLimits(c *gin.Context) {
	var input struct {
		TempWarnLimit float64 `json:"temp_warn_limit" binding:"required"`
		TempOffLimit  float64 `json:"temp_off_limit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Bridge == nil || h.Bridge.FanControl() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fan control not available"})
		return
	}

	if err := h.Bridge.FanControl().SetTempLimits(input.TempWarnLimit, input.TempOffLimit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"temp_warn_limit": input.TempWarnLimit, "temp_off_limit": input.TempOffLimit}})
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

	// Block preset ON if device is overheating
	if h.Bridge != nil && h.Bridge.FanControl() != nil && h.Bridge.FanControl().IsOverheating(dblocker.SerialNumb) {
		c.JSON(http.StatusForbidden, gin.H{"error": "device temperature exceeds safe limit, cannot turn on sectors"})
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

	fanM, fanS := h.fanState(dblocker.SerialNumb, dblocker.PresetConfig)
	bitmaskPayload, err := service.DBlockerConfigToBitmask(
		dblocker.PresetConfig,
		fanM,
		fanS,
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

func (h *DBlockerHandler) SleepDBlocker(c *gin.Context) {
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

	// Turn off all outputs first
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

	fanM, fanS := h.fanState(dblocker.SerialNumb, allOffConfig[:])
	bitmaskPayload, err := service.DBlockerConfigToBitmask(allOffConfig[:], fanM, fanS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offPayload := []byte{byte(bitmaskPayload >> 8), byte(bitmaskPayload)}
	if err := h.MqttClient.Publish(topic, 1, true, offPayload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish off command"})
		return
	}

	// Then send sleep command
	if err := h.MqttClient.Publish(topic, 1, false, []byte("SLEEP")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish sleep command"})
		return
	}

	// Log action
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "sleep",
			DBlockerID:   uint(id),
			DBlockerName: dblocker.Name,
			Config:       allOffConfig[:],
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sleep command sent successfully"})
}

func (h *DBlockerHandler) RebootDBlocker(c *gin.Context) {
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

	topic := fmt.Sprintf("dbl/%s/cmd", dblocker.SerialNumb)

	if err := h.MqttClient.Publish(topic, 1, false, []byte("WAKE_RST")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish reboot command"})
		return
	}

	// Log action
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "reboot",
			DBlockerID:   uint(id),
			DBlockerName: dblocker.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reboot command sent successfully"})
}

func (h *DBlockerHandler) WakeDBlocker(c *gin.Context) {
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

	topic := fmt.Sprintf("dbl/%s/cmd", dblocker.SerialNumb)

	if err := h.MqttClient.Publish(topic, 1, false, []byte("WAKE")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish wake command"})
		return
	}

	// Log action
	user, _ := c.Get("user")
	username := "unknown"
	if u, ok := user.(*models.User); ok {
		username = u.Username
	}
	if h.LogRepo != nil {
		_ = h.LogRepo.Create(&models.ActionLog{
			Username:     username,
			Action:       "wake",
			DBlockerID:   uint(id),
			DBlockerName: dblocker.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Wake command sent successfully"})
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
