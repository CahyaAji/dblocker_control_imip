package route

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/middleware"
	"dblocker_control/internal/service"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterHTTPRoutes(r *gin.Engine, db *gorm.DB, mqttClient mqtt.Client, bridgeHandler *handlerhttp.BridgeHandler, bridgeService *service.BridgeService, authService *service.AuthService) {

	dblockerRepo := repository.NewDBlockerRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	actionLogRepo := repository.NewActionLogRepository(db)
	detectorRepo := repository.NewDetectorRepository(db)
	droneEventRepo := repository.NewDroneEventRepository(db)

	dblockerHandler := handlerhttp.NewDBlockerHandler(dblockerRepo, actionLogRepo, mqttClient, bridgeService)
	authHandler := handlerhttp.NewAuthHandler(authService)
	scheduleHandler := handlerhttp.NewScheduleHandler(scheduleRepo, actionLogRepo, dblockerRepo)
	actionLogHandler := handlerhttp.NewActionLogHandler(actionLogRepo)
	detectorHandler := handlerhttp.NewDetectorHandler(detectorRepo, droneEventRepo)

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
	api.GET("/dblockers/fan-thresholds", dblockerHandler.GetFanThresholds)
	api.PUT("/dblockers/fan-thresholds", dblockerHandler.UpdateFanThresholds)
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
