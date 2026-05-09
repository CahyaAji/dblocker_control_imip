package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/icholy/digest"
)

// Camera holds the configuration for a single Hikvision camera unit.
type Camera struct {
	Host          string // IP address, e.g. "10.88.5.100"
	Port          int    // HTTP/ISAPI port, default 80
	RTSPPort      int    // RTSP port, default 554
	Channel       int    // ISAPI channel number, default 1
	Username      string
	Password      string
	UseMainStream bool // true = use main stream for MJPEG (thermal cameras often lack sub-stream)

	clientOnce      sync.Once
	client          *http.Client
	broadcasterOnce sync.Once
	broadcaster     *StreamBroadcaster

	// detectBroadcaster is created lazily by AttachDetector + GetDetectBroadcaster.
	detectMu          sync.Mutex
	detectBroadcaster *DetectionBroadcaster
}

// GetBroadcaster returns the shared StreamBroadcaster for this camera.
// The broadcaster is created lazily on first call.
func (c *Camera) GetBroadcaster() *StreamBroadcaster {
	c.broadcasterOnce.Do(func() {
		c.broadcaster = newStreamBroadcaster(c)
	})
	return c.broadcaster
}

// AttachDetector wires a YOLO detector to this camera so that
// GetDetectBroadcaster returns annotated frames. Must be called once before
// the first GetDetectBroadcaster call. Pass nil to disable detection.
func (c *Camera) AttachDetector(d *Detector, jpegQuality int) {
	c.detectMu.Lock()
	defer c.detectMu.Unlock()
	if d == nil {
		c.detectBroadcaster = nil
		return
	}
	c.detectBroadcaster = newDetectionBroadcaster(c, d, jpegQuality)
}

// GetDetectBroadcaster returns the annotated-frame broadcaster, or nil if no
// detector has been attached to this camera.
func (c *Camera) GetDetectBroadcaster() *DetectionBroadcaster {
	c.detectMu.Lock()
	defer c.detectMu.Unlock()
	return c.detectBroadcaster
}

// Device represents one physical camera mount with 4 separate IPs:
//   - NormalCam:  normal (visible light) video stream
//   - ThermalCam: thermal video stream
//   - PanTiltCtrl: ISAPI target for pan & tilt commands
//   - ZoomCtrl:    ISAPI target for zoom commands
type Device struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"` // physical mount latitude  (0 = not set)
	Lng         float64 `json:"lng"` // physical mount longitude (0 = not set)
	NormalCam   *Camera
	ThermalCam  *Camera
	PanTiltCtrl *Camera
	ZoomCtrl    *Camera

	// recorder state — managed by recorder.go
	recorderMu sync.Mutex
	recorder   *activeRecording

	// LastAzimuth is the last absolute azimuth (ISAPI tenths-of-degrees) sent
	// to this device. Stored atomically so ListDevices can read it safely.
	LastAzimuth atomic.Int32
}

// RTSPMainStreamURL returns the main-stream RTSP URL for the camera.
func (c *Camera) RTSPMainStreamURL() string {
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/ISAPI/Streaming/channels/%d01",
		c.Username, c.Password, c.Host, c.RTSPPort, c.Channel)
}

// RTSPSubStreamURL returns the sub-stream RTSP URL for the camera.
func (c *Camera) RTSPSubStreamURL() string {
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/ISAPI/Streaming/channels/%d02",
		c.Username, c.Password, c.Host, c.RTSPPort, c.Channel)
}

// StreamURLs returns both stream URLs for the camera.
func (c *Camera) StreamURLs() (main, sub string) {
	return c.RTSPMainStreamURL(), c.RTSPSubStreamURL()
}

// isapiURL builds an ISAPI endpoint URL.
func (c *Camera) isapiURL(path string) string {
	return fmt.Sprintf("http://%s:%d%s", c.Host, c.Port, path)
}

// isapiClient returns the shared HTTP client configured with Digest Auth for this camera.
// The client is created once and reused so the underlying TCP connection pool is shared
// across all ISAPI calls, avoiding a new connection per request.
func (c *Camera) isapiClient() *http.Client {
	c.clientOnce.Do(func() {
		c.client = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &digest.Transport{
				Username: c.Username,
				Password: c.Password,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		}
	})
	return c.client
}

// isapiDo sends an authenticated HTTP request to the camera ISAPI endpoint.
func (c *Camera) isapiDo(method, path, contentType string, body io.Reader) (*http.Response, error) {
	url := c.isapiURL(path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.isapiClient().Do(req)
}

type ptzSpeed struct {
	PanSpeed  int `xml:"panSpeed"`
	TiltSpeed int `xml:"tiltSpeed"`
	ZoomSpeed int `xml:"zoomSpeed"`
}

type ptzContinuousXML struct {
	XMLName xml.Name `xml:"PTZData"`
	Pan     int      `xml:"pan"`
	Tilt    int      `xml:"tilt"`
	Zoom    int      `xml:"zoom"`
	Speed   ptzSpeed `xml:"speed"`
}

// ---- Snapshot ----

// Snapshot fetches a JPEG snapshot from the camera via ISAPI.
func (c *Camera) Snapshot() ([]byte, string, error) {
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d01/picture", c.Channel)
	resp, err := c.isapiDo(http.MethodGet, path, "", nil)
	if err != nil {
		return nil, "", fmt.Errorf("snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("snapshot returned %d: %s", resp.StatusCode, string(b))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("snapshot read: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	return data, ct, nil
}

// ---- Absolute PTZ ----
type PanTiltAbsoluteRequest struct {
	Azimuth   int `json:"azimuth"`
	Elevation int `json:"elevation"`
}

type ptzAbsoluteSet struct {
	Azimuth      int `xml:"azimuth"`
	Elevation    int `xml:"elevation"`
	AbsoluteZoom int `xml:"absoluteZoom"`
}

type ptzAbsoluteXML struct {
	XMLName      xml.Name       `xml:"PTZData"`
	AbsoluteHigh ptzAbsoluteSet `xml:"AbsoluteHigh"`
}

// hikResponseStatus holds the standard Hikvision ResponseStatus XML body.
type hikResponseStatus struct {
	XMLName       xml.Name `xml:"ResponseStatus"`
	RequestURL    string   `xml:"requestURL"`
	StatusCode    string   `xml:"statusCode"`
	StatusString  string   `xml:"statusString"`
	SubStatusCode string   `xml:"subStatusCode"`
}

func (c *Camera) PTZAbsolute(azimuth int, elevation int) (*hikResponseStatus, int, error) {

	bodyStruct := ptzAbsoluteXML{
		AbsoluteHigh: ptzAbsoluteSet{
			Azimuth:      azimuth,
			Elevation:    elevation,
			AbsoluteZoom: 0,
		},
	}

	xmlBody, err := xml.Marshal(bodyStruct)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ptz absolute: %w", err)
	}

	body := append([]byte(xml.Header), xmlBody...)

	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/absolute", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("ptz absolute: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("ptz absolute read response: %w", err)
	}
	var status hikResponseStatus
	if xmlErr := xml.Unmarshal(respBody, &status); xmlErr == nil {
		return &status, resp.StatusCode, nil
	}
	return nil, resp.StatusCode, nil
}

func (c *Camera) PTZZoomAbsolute(zoom int) (*hikResponseStatus, int, error) {

	bodyStruct := ptzAbsoluteXML{
		AbsoluteHigh: ptzAbsoluteSet{
			Azimuth:      0,
			Elevation:    0,
			AbsoluteZoom: zoom,
		},
	}

	xmlBody, err := xml.Marshal(bodyStruct)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ptz absolute: %w", err)
	}

	body := append([]byte(xml.Header), xmlBody...)

	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/absolute", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("ptz absolute: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("ptz absolute read response: %w", err)
	}
	var status hikResponseStatus
	if xmlErr := xml.Unmarshal(respBody, &status); xmlErr == nil {
		return &status, resp.StatusCode, nil
	}
	return nil, resp.StatusCode, nil

}

// PTZContinuous sends a continuous pan/tilt move command.
// pan and tilt are speeds: negative = left/down, positive = right/up, 0 = stop that axis.
// Call with pan=0, tilt=0 to stop movement.
func (c *Camera) PTZContinuous(pan, tilt int) (*hikResponseStatus, int, error) {
	data := ptzContinuousXML{
		Pan:  pan,
		Tilt: tilt,
		Zoom: 0,
	}
	xmlBody, err := xml.Marshal(data)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ptz continuous: %w", err)
	}
	body := append([]byte(xml.Header), xmlBody...)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/continuous", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("ptz continuous: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("ptz continuous read response: %w", err)
	}
	var status hikResponseStatus
	if xmlErr := xml.Unmarshal(respBody, &status); xmlErr == nil {
		return &status, resp.StatusCode, nil
	}
	return nil, resp.StatusCode, nil
}

// PTZGetAzimuth reads the current pan/tilt azimuth from the camera via ISAPI
// GET /ISAPI/PTZCtrl/channels/{ch}/status.
// Returns the azimuth in ISAPI units (tenths of degrees, 0–3599) or an error.
func (c *Camera) PTZGetAzimuth() (int, error) {
	type ptzStatusHigh struct {
		Azimuth int `xml:"azimuth"`
	}
	type ptzStatus struct {
		XMLName      xml.Name      `xml:"PTZStatus"`
		AbsoluteHigh ptzStatusHigh `xml:"AbsoluteHigh"`
	}
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/status", c.Channel)
	resp, err := c.isapiDo(http.MethodGet, path, "", nil)
	if err != nil {
		return 0, fmt.Errorf("ptz status: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("ptz status read: %w", err)
	}
	var s ptzStatus
	if err := xml.Unmarshal(body, &s); err != nil {
		return 0, fmt.Errorf("ptz status parse: %w", err)
	}
	return s.AbsoluteHigh.Azimuth, nil
}

// PTZZoomContinuous sends a continuous zoom command.
// zoom is speed: positive = zoom in, negative = zoom out, 0 = stop.
func (c *Camera) PTZZoomContinuous(zoom int) (*hikResponseStatus, int, error) {
	data := ptzContinuousXML{
		Pan:  0,
		Tilt: 0,
		Zoom: zoom,
	}
	xmlBody, err := xml.Marshal(data)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ptz zoom continuous: %w", err)
	}
	body := append([]byte(xml.Header), xmlBody...)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/continuous", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("ptz zoom continuous: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("ptz zoom continuous read response: %w", err)
	}
	var status hikResponseStatus
	if xmlErr := xml.Unmarshal(respBody, &status); xmlErr == nil {
		return &status, resp.StatusCode, nil
	}
	return nil, resp.StatusCode, nil
}

// ---- Wiper ----

type ptzAuxXML struct {
	XMLName xml.Name `xml:"PTZAux"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	ID      int      `xml:"id"`
	Type    string   `xml:"type"`
	Status  string   `xml:"status"`
}

// PTZWiper sends a wiper on/off command via ISAPI AuxControl.
// status must be "on" or "off".
func (c *Camera) PTZWiper(wiperStatus string) (*hikResponseStatus, int, error) {
	data := ptzAuxXML{
		Version: "2.0",
		Xmlns:   "http://www.isapi.org/ver20/XMLSchema",
		ID:      1,
		Type:    "WIPER",
		Status:  wiperStatus,
	}
	xmlBody, err := xml.Marshal(data)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal wiper: %w", err)
	}
	body := append([]byte(xml.Header), xmlBody...)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/auxcontrols/1", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("wiper: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("wiper read response: %w", err)
	}
	var hikStatus hikResponseStatus
	if xmlErr := xml.Unmarshal(respBody, &hikStatus); xmlErr == nil {
		return &hikStatus, resp.StatusCode, nil
	}
	return nil, resp.StatusCode, nil
}
