package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DeviceHandler struct {
	devices []*Device
}

func NewDeviceHandler(devices []*Device) *DeviceHandler {
	return &DeviceHandler{devices: devices}
}

// findDevice returns the device with the given id, or nil.
func (h *DeviceHandler) findDevice(idStr string) *Device {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil
	}
	for _, d := range h.devices {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// GET /devices
// Returns the list of devices (without credentials).
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	type deviceInfo struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		NormalIP  string `json:"normal_ip"`
		ThermalIP string `json:"thermal_ip"`
		PanTiltIP string `json:"pantilt_ip"`
		ZoomIP    string `json:"zoom_ip"`
	}
	result := make([]deviceInfo, 0, len(h.devices))
	for _, d := range h.devices {
		result = append(result, deviceInfo{
			ID:        d.ID,
			Name:      d.Name,
			NormalIP:  d.NormalCam.Host,
			ThermalIP: d.ThermalCam.Host,
			PanTiltIP: d.PanTiltCtrl.Host,
			ZoomIP:    d.ZoomCtrl.Host,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GET /devices/:id/rtsp
// Returns the raw RTSP URLs for both cameras (for VLC / ffplay testing).
func (h *DeviceHandler) GetRTSPURLs(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	nMain, nSub := d.NormalCam.StreamURLs()
	tMain, tSub := d.ThermalCam.StreamURLs()
	c.JSON(http.StatusOK, gin.H{
		"normal":  gin.H{"main_stream": nMain, "sub_stream": nSub},
		"thermal": gin.H{"main_stream": tMain, "sub_stream": tSub},
	})
}

// GET /devices/:id/stream/:cam
// Serves a live MJPEG stream over HTTP (cam = normal | thermal).
// Can be used directly as <img src=".../stream/normal"> in any browser.
//
// Future YOLO integration example:
//
//	raw := cam.StartMJPEGStream(ctx)
//	annotated := ProcessFrames(ctx, raw, yoloProcessor)
//	serveStream(c, annotated)
func (h *DeviceHandler) StreamMJPEG(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	cam := d.NormalCam
	if c.Param("cam") == "thermal" {
		cam = d.ThermalCam
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	frames := cam.StartMJPEGStream(ctx)

	const boundary = "mjpegframe"
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	w := c.Writer
	for frame := range frames {
		fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
		w.Write(frame)
		fmt.Fprintf(w, "\r\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// GET /devices/:id/snapshot?cam=normal|thermal
// Returns a JPEG snapshot directly from the camera. Defaults to normal.
func (h *DeviceHandler) Snapshot(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	cam := d.NormalCam
	if c.Query("cam") == "thermal" {
		cam = d.ThermalCam
	}
	data, ct, err := cam.Snapshot()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, ct, data)
}

// POST /devices/:id/ptz/absolute
// Body: { "azimuth": 0-3600, "elevation": -900..900, "absolute_zoom": 0-1000 }
// Moves the pan/tilt camera to an absolute position via ISAPI.
func (h *DeviceHandler) PanTiltAbsolute(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req PTZAbsoluteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	status, upstreamCode, err := d.PTZAbsolute(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	upstreamBody := gin.H{}
	if status != nil && (status.RequestURL != "" || status.StatusCode != "" || status.StatusString != "" || status.SubStatusCode != "") {
		upstreamBody = gin.H{
			"request_url":     status.RequestURL,
			"status_code":     status.StatusCode,
			"status_string":   status.StatusString,
			"sub_status_code": status.SubStatusCode,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":              upstreamCode >= 200 && upstreamCode < 300,
		"device_id":       d.ID,
		"target_ip":       d.PanTiltCtrl.Host,
		"upstream_status": upstreamCode,
		"upstream_body":   upstreamBody,
	})
}

// POST /devices/:id/ptz
// Body: { "pan": 0, "tilt": 0, "zoom": 0 }
// Pan/tilt routed to Normal camera, zoom routed to Thermal camera.
// Send all zeros to stop. Values are -100..100.
func (h *DeviceHandler) PTZControl(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req PTZContinuousRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := d.PTZContinuous(req); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// POST /devices/:id/ptz/stop
// Stops all PTZ movement on both cameras.
func (h *DeviceHandler) PTZStop(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if err := d.PTZStop(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// POST /devices/:id/ptz/preset/:preset
// Moves both cameras to a saved preset position.
func (h *DeviceHandler) PTZGotoPreset(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	presetID, err := strconv.Atoi(c.Param("preset"))
	if err != nil || presetID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid preset id"})
		return
	}
	if err := d.PTZGotoPreset(presetID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
