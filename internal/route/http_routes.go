package route

import (
	"context"
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/middleware"
	"dblocker_control/internal/service"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterHTTPRoutes(r *gin.Engine, db *gorm.DB, mqttClient mqtt.Client, bridgeHandler *handlerhttp.BridgeHandler, bridgeService *service.BridgeService, authService *service.AuthService, sleepSchedule *service.SleepScheduleService) {

	dblockerRepo := repository.NewDBlockerRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	actionLogRepo := repository.NewActionLogRepository(db)
	detectorRepo := repository.NewDetectorRepository(db)
	droneEventRepo := repository.NewDroneEventRepository(db)
	appSettingRepo := repository.NewAppSettingRepository(db)

	dblockerHandler := handlerhttp.NewDBlockerHandler(dblockerRepo, actionLogRepo, mqttClient, bridgeService, sleepSchedule)
	authHandler := handlerhttp.NewAuthHandler(authService)
	scheduleHandler := handlerhttp.NewScheduleHandler(scheduleRepo, actionLogRepo, dblockerRepo)
	actionLogHandler := handlerhttp.NewActionLogHandler(actionLogRepo)
	detectorHandler := handlerhttp.NewDetectorHandlerWithSettings(detectorRepo, droneEventRepo, appSettingRepo)

	// Public routes
	r.POST("/api/auth/login", authHandler.Login)

	// SSE - protected
	r.GET("/events", middleware.AuthRequired(authService), bridgeHandler.Events)

	api := r.Group("/api")
	api.Use(middleware.AuthRequired(authService))

	// Auth
	api.GET("/auth/me", authHandler.Me)

	// User management - admin only
	admin := api.Group("/users")
	admin.Use(middleware.AdminRequired())
	admin.POST("", authHandler.CreateUser)
	admin.GET("", authHandler.ListUsers)
	admin.DELETE("/:id", authHandler.DeleteUser)

	// DBlockers
	api.POST("/dblockers", dblockerHandler.CreateDBlocker)
	api.GET("/dblockers", dblockerHandler.GetDBlockers)
	api.GET("/dblockers/:id", dblockerHandler.GetDBlockerByID)
	api.PUT("/dblockers/:id", dblockerHandler.UpdateDBlocker)
	api.PUT("/dblockers/config", dblockerHandler.UpdateDBlockerConfig)
	api.GET("/dblockers/config/off/:id", dblockerHandler.TurnOffAllDBlockerConfig)
	api.GET("/dblockers/config/preset/:id", dblockerHandler.PresetOnDBlockerConfig)
	api.POST("/dblockers/sleep/:id", dblockerHandler.SleepDBlocker)
	api.POST("/dblockers/reboot/:id", dblockerHandler.RebootDBlocker)
	api.POST("/dblockers/wake/:id", dblockerHandler.WakeDBlocker)
	api.PUT("/dblockers/preset", dblockerHandler.UpdatePresetConfig)
	api.GET("/dblockers/monitor", dblockerHandler.GetMonitorStatus)
	api.GET("/dblockers/monitor-settings", dblockerHandler.GetMonitorSettings)
	api.PUT("/dblockers/monitor-settings", dblockerHandler.UpdateMonitorSettings)
	api.GET("/dblockers/fan-thresholds", dblockerHandler.GetFanThresholds)
	api.PUT("/dblockers/fan-thresholds", dblockerHandler.UpdateFanThresholds)
	api.GET("/dblockers/temp-limits", dblockerHandler.GetTempLimits)
	api.PUT("/dblockers/temp-limits", dblockerHandler.UpdateTempLimits)
	api.GET("/dblockers/sleep-schedule", dblockerHandler.GetSleepSchedule)
	api.PUT("/dblockers/sleep-schedule", dblockerHandler.UpdateSleepSchedule)
	api.DELETE("/dblockers/:id", dblockerHandler.DeleteDBlocker)

	// Schedules
	api.POST("/schedules", scheduleHandler.CreateSchedule)
	api.GET("/schedules", scheduleHandler.GetSchedules)
	api.PUT("/schedules/:id/toggle", scheduleHandler.ToggleSchedule)
	api.DELETE("/schedules/:id", scheduleHandler.DeleteSchedule)

	// Action Logs - admin only for reading, service can create
	api.GET("/logs", middleware.AdminRequired(), actionLogHandler.GetLogs)
	api.DELETE("/logs/:id", middleware.AdminRequired(), actionLogHandler.DeleteLog)
	api.POST("/logs", actionLogHandler.CreateLog)

	// Drone Detectors
	api.POST("/detectors", detectorHandler.CreateDetector)
	api.GET("/detectors", detectorHandler.GetDetectors)
	api.PUT("/detectors/:id", detectorHandler.UpdateDetector)
	api.DELETE("/detectors/:id", detectorHandler.DeleteDetector)

	// Drone Events
	api.GET("/drone-events", detectorHandler.GetDroneEvents)
	api.POST("/drone-events", detectorHandler.CreateDroneEvent)
	api.DELETE("/drone-events", detectorHandler.DeleteDroneEventsByDate)
	api.PUT("/detectors/status", detectorHandler.UpdateDetectorStatus)
	api.GET("/detectors/settings", detectorHandler.GetDetectionSettings)
	api.PUT("/detectors/settings", detectorHandler.UpdateDetectionSettings)

	// Vision proxy: forward /cam/* to the vision server
	visionURL := os.Getenv("VISION_URL")
	if visionURL == "" {
		visionURL = "http://dblocker-vision:8090"
	}
	visionTarget, err := url.Parse(visionURL)
	if err != nil {
		panic("invalid VISION_URL: " + err.Error())
	}
	visionProxy := httputil.NewSingleHostReverseProxy(visionTarget)
	// Use a transport with a short dial timeout so a down vision server fails
	// fast instead of holding the connection for ~90 s (which would starve the
	// SSE /events stream and make the dblocker list appear empty).
	visionProxy.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		},
		ResponseHeaderTimeout: 10 * time.Second,
	}
	visionProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("vision proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "vision server unavailable"})
	}
	r.Any("/cam/*path", func(c *gin.Context) {
		visionProxy.ServeHTTP(c.Writer, c.Request)
	})

	//! make sure frontend is built first: npm run build (inside frontend/)

	frontendDist := resolveFrontendDistPath()
	if frontendDist != "" {
		r.Static("/assets", filepath.Join(frontendDist, "assets"))
		r.GET("/dashboard", func(ctx *gin.Context) {
			ctx.File(filepath.Join(frontendDist, "index.html"))
		})
		r.GET("/logs", func(ctx *gin.Context) {
			ctx.File(filepath.Join(frontendDist, "logs.html"))
		})
		r.GET("/detections", func(ctx *gin.Context) {
			ctx.File(filepath.Join(frontendDist, "detections.html"))
		})
		r.GET("/camera", func(ctx *gin.Context) {
			ctx.File(filepath.Join(frontendDist, "camera.html"))
		})
	} else {
		r.GET("/dashboard", func(ctx *gin.Context) {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "frontend build not found. run: cd frontend && npm run build",
			})
		})
	}
}

func resolveFrontendDistPath() string {
	candidates := []string{
		"frontend/dist",
		"../frontend/dist",
		"/app/frontend/dist",
	}

	for _, candidate := range candidates {
		indexFile := filepath.Join(candidate, "index.html")
		if _, err := os.Stat(indexFile); err == nil {
			return candidate
		}
	}

	return ""
}
