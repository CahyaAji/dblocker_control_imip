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

	dblockerHandler := handlerhttp.NewDBlockerHandler(dblockerRepo, mqttClient, bridgeService)
	authHandler := handlerhttp.NewAuthHandler(authService)
	scheduleHandler := handlerhttp.NewScheduleHandler(scheduleRepo)

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
	api.PUT("/dblockers/preset", dblockerHandler.UpdatePresetConfig)
	api.GET("/dblockers/monitor", dblockerHandler.GetMonitorStatus)
	api.DELETE("/dblockers/:id", dblockerHandler.DeleteDBlocker)

	// Schedules
	api.POST("/schedules", scheduleHandler.CreateSchedule)
	api.GET("/schedules", scheduleHandler.GetSchedules)
	api.PUT("/schedules/:id/toggle", scheduleHandler.ToggleSchedule)
	api.DELETE("/schedules/:id", scheduleHandler.DeleteSchedule)

	//! make sure frontend is built first: npm run build (inside frontend/)

	frontendDist := resolveFrontendDistPath()
	if frontendDist != "" {
		r.Static("/assets", filepath.Join(frontendDist, "assets"))
		r.GET("/dashboard", func(ctx *gin.Context) {
			ctx.File(filepath.Join(frontendDist, "index.html"))
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
