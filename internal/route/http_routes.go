package route

import (
	handlerhttp "dblocker_control/internal/handler/http"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
)

func RegisterHTTPRoutes(r *gin.Engine, bridgeHandler *handlerhttp.BridgeHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(stdhttp.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/events", bridgeHandler.Events)

	//! uncomment this if you want to serve the frontend from the same server, but make sure to build the frontend first

	// r.Static("/assets", "./frontend/dist/assets")
	// r.GET("/dashboard", func(ctx *gin.Context) {
	// 	ctx.File("./frontend/dist/index.html")
	// })
}
