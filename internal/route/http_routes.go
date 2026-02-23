package route

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"

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

	//! uncomment this if you want to serve the frontend from the same server, but make sure to build the frontend first

	// r.Static("/assets", "./frontend/dist/assets")
	// r.GET("/dashboard", func(ctx *gin.Context) {
	// 	ctx.File("./frontend/dist/index.html")
	// })
}
