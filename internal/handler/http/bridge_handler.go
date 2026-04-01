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
	sub, snapshot := h.bridge.SubscribeWithSnapshot()
	defer h.bridge.Unsubscribe(sub)

	// Flush retained /sta snapshot first, then multiplex live channels.
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

		// Multiplex: prefer /sta (status) over /rpt (sensor data).
		select {
		case msg, ok := <-sub.StaCh:
			if !ok {
				return false
			}
			c.SSEvent("message", gin.H{
				"topic":   msg.Topic,
				"payload": string(msg.Payload),
			})
			return true
		default:
		}

		// No /sta pending — wait for either channel.
		select {
		case msg, ok := <-sub.StaCh:
			if !ok {
				return false
			}
			c.SSEvent("message", gin.H{
				"topic":   msg.Topic,
				"payload": string(msg.Payload),
			})
			return true
		case msg, ok := <-sub.LiveCh:
			if !ok {
				return false
			}
			c.SSEvent("message", gin.H{
				"topic":   msg.Topic,
				"payload": string(msg.Payload),
			})
			return true
		}
	})
}
