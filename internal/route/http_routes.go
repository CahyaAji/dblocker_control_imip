package route

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterHTTPRoutes(r *gin.Engine, db *gorm.DB, mqttClient mqtt.Client, bridgeHandler *handlerhttp.BridgeHandler) {

	dblockerRepo := repository.NewDBlockerRepository(db)

	dblockerHandler := handlerhttp.NewDBlockerHandler(dblockerRepo, mqttClient)

	r.GET("/events", bridgeHandler.Events)

	api := r.Group("/api")

	// DBlockers
	api.POST("/dblockers", dblockerHandler.CreateDBlocker)
	api.GET("/dblockers", dblockerHandler.GetDBlockers)
	api.GET("/dblockers/:id", dblockerHandler.GetDBlockerByID)
	api.PUT("/dblockers/:id", dblockerHandler.UpdateDBlocker)
	api.PUT("/dblockers/config", dblockerHandler.UpdateDBlockerConfig)
	api.DELETE("/dblockers/:id", dblockerHandler.DeleteDBlocker)

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
