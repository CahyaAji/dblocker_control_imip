package main

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
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
}

// Device represents one physical camera mount with 4 separate IPs:
//   - NormalCam:  normal (visible light) video stream
//   - ThermalCam: thermal video stream
//   - PanTiltCtrl: ISAPI target for pan & tilt commands
//   - ZoomCtrl:    ISAPI target for zoom commands
type Device struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	NormalCam   *Camera
	ThermalCam  *Camera
	PanTiltCtrl *Camera
	ZoomCtrl    *Camera
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

// isapiClient returns an HTTP client configured with Digest Auth for this camera.
// Each camera gets its own transport so credentials are per-camera.
func (c *Camera) isapiClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
		Transport: &digest.Transport{
			Username: c.Username,
			Password: c.Password,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
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

// ---- PTZ ----

// PTZContinuousRequest holds speed values for continuous PTZ movement.
// Pan/Tilt/Zoom range: -100 to 100 (0 = stop).
type PTZContinuousRequest struct {
	Pan  int `json:"pan"`  // negative = left, positive = right
	Tilt int `json:"tilt"` // negative = down, positive = up
	Zoom int `json:"zoom"` // negative = wide, positive = tele
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

// ---- Device-level PTZ (routes pan/tilt → Normal, zoom → Thermal) ----

// PTZContinuous routes a combined PTZ command:
// - Pan & Tilt → PanTiltCtrl camera
// - Zoom       → ZoomCtrl camera
// Call with all zeros to stop both.
func (d *Device) PTZContinuous(req PTZContinuousRequest) error {
	var firstErr error
	if err := d.PanTiltCtrl.ptzContinuous(req.Pan, req.Tilt, 0); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := d.ZoomCtrl.ptzContinuous(0, 0, req.Zoom); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// PTZStop stops movement on both control cameras.
func (d *Device) PTZStop() error {
	return d.PTZContinuous(PTZContinuousRequest{})
}

// PTZGotoPreset moves both control cameras to a saved preset.
func (d *Device) PTZGotoPreset(presetID int) error {
	var firstErr error
	if err := d.PanTiltCtrl.ptzGotoPreset(presetID); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := d.ZoomCtrl.ptzGotoPreset(presetID); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// ---- Camera-level PTZ (internal) ----

// ptzContinuous sends a continuous PTZ move command to this camera.
// Call with all zeros to stop movement.
func (c *Camera) ptzContinuous(pan, tilt, zoom int) error {
	data := ptzContinuousXML{
		Pan:  pan,
		Tilt: tilt,
		Zoom: zoom,
		Speed: ptzSpeed{
			PanSpeed:  abs(pan),
			TiltSpeed: abs(tilt),
			ZoomSpeed: abs(zoom),
		},
	}
	xmlBody, err := xml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal ptz: %w", err)
	}

	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/continuous", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytesReader(xmlBody))
	if err != nil {
		return fmt.Errorf("ptz continuous: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ptz continuous returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ptzGotoPreset moves this camera to a saved preset position.
func (c *Camera) ptzGotoPreset(presetID int) error {
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/presets/%d/goto", c.Channel, presetID)
	resp, err := c.isapiDo(http.MethodPut, path, "", nil)
	if err != nil {
		return fmt.Errorf("ptz goto preset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ptz goto preset returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ---- Absolute PTZ ----

// PTZAbsoluteRequest holds absolute position values.
// Azimuth: 0–3600 (tenths of a degree, 0 = north/home).
// Elevation: -900..900 (tenths of a degree).
// AbsoluteZoom: 0..1000.
type PTZAbsoluteRequest struct {
	Azimuth      int     `json:"azimuth"`
	Elevation    int     `json:"elevation"`
	AbsoluteZoom float64 `json:"absolute_zoom"`
}

type ptzAbsoluteSet struct {
	Azimuth      int     `xml:"azimuth"`
	Elevation    int     `xml:"elevation"`
	AbsoluteZoom float64 `xml:"absoluteZoom"`
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

// ptzAbsolute sends an absolute position command to this camera.
func (c *Camera) ptzAbsolute(azimuth, elevation int, absoluteZoom float64) (*hikResponseStatus, int, error) {
	data := ptzAbsoluteXML{
		AbsoluteHigh: ptzAbsoluteSet{
			Azimuth:      azimuth,
			Elevation:    elevation,
			AbsoluteZoom: absoluteZoom,
		},
	}
	xmlBody, err := xml.Marshal(data)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ptz absolute: %w", err)
	}
	body := append([]byte(xml.Header), xmlBody...)
	path := fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/absolute", c.Channel)
	resp, err := c.isapiDo(http.MethodPut, path, "application/xml", bytesReader(body))
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

// PTZAbsolute moves the PanTiltCtrl camera to an absolute position.
func (d *Device) PTZAbsolute(req PTZAbsoluteRequest) (*hikResponseStatus, int, error) {
	return d.PanTiltCtrl.ptzAbsolute(req.Azimuth, req.Elevation, req.AbsoluteZoom)
}

// ---- helpers ----

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func bytesReader(b []byte) io.Reader {
	return &bytesReadCloser{data: b, pos: 0}
}

type bytesReadCloser struct {
	data []byte
	pos  int
}

func (br *bytesReadCloser) Read(p []byte) (int, error) {
	if br.pos >= len(br.data) {
		return 0, io.EOF
	}
	n := copy(p, br.data[br.pos:])
	br.pos += n
	return n, nil
}
