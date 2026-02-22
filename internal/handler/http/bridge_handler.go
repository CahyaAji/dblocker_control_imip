package http

import (
	"dblocker_control/internal/service"
	"io"

	"github.com/gin-gonic/gin"
)

// BridgeHandler exposes SSE endpoints for MQTT bridge traffic.
type BridgeHandler struct {
	bridge *service.BridgeService
}

func NewBridgeHandler(bridge *service.BridgeService) *BridgeHandler {
	return &BridgeHandler{bridge: bridge}
}

// Events streams MQTT messages to browsers using SSE.
func (h *BridgeHandler) Events(c *gin.Context) {
	ch := h.bridge.Subscribe()
	defer h.bridge.Unsubscribe(ch)

	c.Stream(func(w io.Writer) bool {
		msg, ok := <-ch
		if !ok {
			return false
		}
		c.SSEvent("message", gin.H{
			"topic":   msg.Topic,
			"payload": string(msg.Payload),
		})
		return true
	})
}
