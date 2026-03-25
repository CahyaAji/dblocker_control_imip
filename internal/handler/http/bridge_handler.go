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
	snapshot := h.bridge.Snapshot()
	ch := h.bridge.Subscribe()
	defer h.bridge.Unsubscribe(ch)

	// Flush retained /sta snapshot first (never touches the channel buffer),
	// then block on live messages. This way channel size is independent of
	// the number of devices.
	snapshotIdx := 0
	c.Stream(func(w io.Writer) bool {
		if snapshotIdx < len(snapshot) {
			msg := snapshot[snapshotIdx]
			snapshotIdx++
			c.SSEvent("message", gin.H{
				"topic":   msg.Topic,
				"payload": string(msg.Payload),
			})
			return true
		}

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
