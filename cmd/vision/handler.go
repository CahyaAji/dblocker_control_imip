package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	internalmqtt "dblocker_control/internal/infrastructure/mqtt"

	"github.com/gin-gonic/gin"
)

// mqttPub is set by main() after MQTT connects. If nil, publishing is skipped.
var mqttPub internalmqtt.Client

type DeviceHandler struct {
	devices   []*Device
	recordDir string
}

func NewDeviceHandler(devices []*Device, recordDir string) *DeviceHandler {
	return &DeviceHandler{devices: devices, recordDir: recordDir}
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
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		Lat         float64 `json:"lat"`
		Lng         float64 `json:"lng"`
		LastAzimuth int     `json:"last_azimuth"`
		NormalIP    string  `json:"normal_ip"`
		ThermalIP   string  `json:"thermal_ip"`
		PanTiltIP   string  `json:"pantilt_ip"`
		ZoomIP      string  `json:"zoom_ip"`
	}
	result := make([]deviceInfo, 0, len(h.devices))
	for _, d := range h.devices {
		result = append(result, deviceInfo{
			ID:          d.ID,
			Name:        d.Name,
			Lat:         d.Lat,
			Lng:         d.Lng,
			LastAzimuth: int(d.LastAzimuth.Load()),
			NormalIP:    d.NormalCam.Host,
			ThermalIP:   d.ThermalCam.Host,
			PanTiltIP:   d.PanTiltCtrl.Host,
			ZoomIP:      d.ZoomCtrl.Host,
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
// All viewers and any active recorder share a single FFmpeg process via
// the per-camera StreamBroadcaster. YOLO frame processing (future) will
// slot in between the broadcaster and the subscribers automatically.
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

	subID, frames := cam.GetBroadcaster().Subscribe()
	defer cam.GetBroadcaster().Unsubscribe(subID)

	const boundary = "mjpegframe"
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	w := c.Writer
	reqCtx := c.Request.Context()
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return
			}
			fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
			w.Write(frame)
			fmt.Fprintf(w, "\r\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-reqCtx.Done():
			return
		}
	}
}

// GET /devices/:id/stream/:cam/detect
// Serves an MJPEG stream of the requested camera with YOLO bounding boxes drawn.
// Only supported for cam=normal. A single inference goroutine per device is shared
// across all viewers and stops automatically when the last viewer disconnects.
func (h *DeviceHandler) StreamMJPEGDetect(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if c.Param("cam") != "normal" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "detection is only available for the normal camera"})
		return
	}
	bc := d.NormalCam.GetDetectBroadcaster()
	if bc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "detector not configured (set DETECT_MODEL_PATH)"})
		return
	}

	subID, frames := bc.Subscribe()
	defer bc.Unsubscribe(subID)

	const boundary = "mjpegframe"
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	w := c.Writer
	reqCtx := c.Request.Context()
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return
			}
			fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
			w.Write(frame)
			fmt.Fprintf(w, "\r\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-reqCtx.Done():
			return
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
// Body: { "azimuth": 0-3600, "elevation": -900..900 }
// Moves the pan/tilt camera to an absolute position via ISAPI.
func (h *DeviceHandler) PanTiltAbsolute(c *gin.Context) {

	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req PanTiltAbsoluteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	status, upstreamCode, err := d.PanTiltCtrl.PTZAbsolute(req.Azimuth, req.Elevation)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	d.LastAzimuth.Store(int32(req.Azimuth))
	// Publish heading update so the map marker stays in sync.
	if mqttPub != nil {
		if msg, err := json.Marshal(map[string]int{"azimuth": req.Azimuth}); err == nil {
			_ = mqttPub.Publish(fmt.Sprintf("cam/%d/heading", d.ID), 0, true, string(msg))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id":     d.ID,
		"pantilt_ip":    d.PanTiltCtrl.Host,
		"azimuth":       req.Azimuth,
		"elevation":     req.Elevation,
		"status":        status.StatusCode,
		"upstream_code": upstreamCode,
	})
}

type ZoomAbsoluteRequest struct {
	Zoom int `json:"zoom"`
}

// max zoom 320
func (h *DeviceHandler) ZoomAbsolute(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req ZoomAbsoluteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	status, upstreamCode, err := d.ZoomCtrl.PTZZoomAbsolute(req.Zoom)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id":     d.ID,
		"zoom_ip":       d.ZoomCtrl.Host,
		"zoom":          req.Zoom,
		"status":        status.StatusCode,
		"upstream_code": upstreamCode,
	})
}

type PanTiltContinuousRequest struct {
	Pan  int `json:"pan"`
	Tilt int `json:"tilt"`
}

// POST /devices/:id/pantilt/continuous
// Body: { "pan": -100..100, "tilt": -100..100 }
// Starts continuous pan/tilt. Send pan=0, tilt=0 to stop.
func (h *DeviceHandler) PanTiltContinuous(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req PanTiltContinuousRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	status, upstreamCode, err := d.PanTiltCtrl.PTZContinuous(req.Pan, req.Tilt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	// When movement stops (pan=0 tilt=0), read the real azimuth from ISAPI
	// after a short settle delay and publish it to MQTT.
	if req.Pan == 0 && req.Tilt == 0 && mqttPub != nil {
		devID := d.ID
		cam := d.PanTiltCtrl
		go func() {
			time.Sleep(400 * time.Millisecond)
			azimuth, err := cam.PTZGetAzimuth()
			if err != nil {
				log.Printf("warn: PTZGetAzimuth device %d: %v", devID, err)
				return
			}
			d.LastAzimuth.Store(int32(azimuth))
			if msg, err := json.Marshal(map[string]int{"azimuth": azimuth}); err == nil {
				_ = mqttPub.Publish(fmt.Sprintf("cam/%d/heading", devID), 0, true, string(msg))
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id":     d.ID,
		"pantilt_ip":    d.PanTiltCtrl.Host,
		"pan":           req.Pan,
		"tilt":          req.Tilt,
		"status":        status.StatusCode,
		"upstream_code": upstreamCode,
	})
}

type ZoomContinuousRequest struct {
	Zoom int `json:"zoom"`
}

// POST /devices/:id/zoom/continuous
// Body: { "zoom": -100..100 }
// Starts continuous zoom. Send zoom=0 to stop.
func (h *DeviceHandler) ZoomContinuous(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req ZoomContinuousRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	status, upstreamCode, err := d.ZoomCtrl.PTZZoomContinuous(req.Zoom)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id":     d.ID,
		"zoom_ip":       d.ZoomCtrl.Host,
		"zoom":          req.Zoom,
		"status":        status.StatusCode,
		"upstream_code": upstreamCode,
	})
}

// ── Recording ─────────────────────────────────────────────────────────────

// POST /devices/:id/record/start
// Body: { "cam": "normal"|"thermal", "duration": <seconds> }
// duration is optional; defaults to the server maximum (10 min).
// Also usable without the UI for programmatic triggering.
func (h *DeviceHandler) RecordStart(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	var req struct {
		Cam      string  `json:"cam"`
		Detect   bool    `json:"detect"`
		Duration float64 `json:"duration"` // seconds; 0 = use max
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	if req.Cam == "" {
		req.Cam = "normal"
	}
	dur := time.Duration(req.Duration * float64(time.Second))
	if err := d.RecordStart(req.Cam, req.Detect, dur, h.recordDir); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started", "cam": req.Cam, "detect": req.Detect})
}

// POST /devices/:id/record/stop
func (h *DeviceHandler) RecordStop(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if err := d.RecordStop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// GET /devices/:id/record/status
func (h *DeviceHandler) RecordStatus(c *gin.Context) {
	d := h.findDevice(c.Param("id"))
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	recording, cam, filename, detect, startedAt := d.RecordStatus()
	if !recording {
		c.JSON(http.StatusOK, gin.H{"recording": false})
		return
	}
	elapsed := time.Since(startedAt).Seconds()
	remaining := maxRecordDuration.Seconds() - elapsed
	if remaining < 0 {
		remaining = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"recording":         true,
		"cam":               cam,
		"detect":            detect,
		"filename":          filename,
		"started_at":        startedAt.Format(time.RFC3339),
		"elapsed_seconds":   int(elapsed),
		"remaining_seconds": int(remaining),
	})
}
