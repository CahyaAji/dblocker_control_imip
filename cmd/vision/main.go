package main

import (
	"log"
	"os"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	devices := loadDevicesFromEnv()
	if len(devices) == 0 {
		log.Println("WARNING: no devices configured. Set DEVICE_n_NORMAL_HOST env vars.")
	}

	appPort := getEnv("APP_PORT", "8090")
	recordDir := getEnv("RECORD_DIR", "/recordings")

	r := gin.Default()
	r.Use(cors.Default())

	h := NewDeviceHandler(devices, recordDir)

	cam := r.Group("/cam")
	{
		cam.GET("/devices", h.ListDevices)
		cam.GET("/devices/:id/rtsp", h.GetRTSPURLs)
		cam.GET("/devices/:id/stream/:cam", h.StreamMJPEG)
		cam.GET("/devices/:id/snapshot", h.Snapshot)
		// cam.POST("/devices/:id/ptz", h.PTZControl)
		// cam.POST("/devices/:id/ptz/stop", h.PTZStop)
		cam.POST("/devices/:id/pantilt", h.PanTiltAbsolute)
		cam.POST("/devices/:id/pantilt/continuous", h.PanTiltContinuous)
		cam.POST("/devices/:id/zoom", h.ZoomAbsolute)
		cam.POST("/devices/:id/zoom/continuous", h.ZoomContinuous)
		// cam.POST("/devices/:id/ptz/preset/:preset", h.PTZGotoPreset)
		cam.POST("/devices/:id/record/start", h.RecordStart)
		cam.POST("/devices/:id/record/stop", h.RecordStop)
		cam.GET("/devices/:id/record/status", h.RecordStatus)
	}

	log.Printf("Vision server starting on :%s", appPort)
	if err := r.Run(":" + appPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadDevicesFromEnv reads device configs from environment variables.
// Each device requires 4 hosts:
//
//	DEVICE_1_NAME=Device 1
//	DEVICE_1_NORMAL_HOST=10.88.5.100    (normal/visible stream)
//	DEVICE_1_THERMAL_HOST=10.88.5.101   (thermal stream)
//	DEVICE_1_THERMAL_MAIN_STREAM=true   (optional, use main stream for thermal — default false)
//	DEVICE_1_PANTILT_HOST=10.88.5.100   (pan & tilt ISAPI control)
//	DEVICE_1_ZOOM_HOST=10.88.5.101      (zoom ISAPI control)
//	DEVICE_1_PORT=80                    (optional, default 80)
//	DEVICE_1_RTSP_PORT=554              (optional, default 554)
//	DEVICE_1_CHANNEL=1                  (optional, default 1)
//
// Shared credentials:
//
//	CAMERA_USERNAME=admin
//	CAMERA_PASSWORD=secret
func loadDevicesFromEnv() []*Device {
	username := getEnv("CAMERA_USERNAME", "admin")
	password := os.Getenv("CAMERA_PASSWORD")

	devices := make([]*Device, 0, 4)
	for i := 1; i <= 8; i++ {
		prefix := "DEVICE_" + strconv.Itoa(i) + "_"
		normalHost := os.Getenv(prefix + "NORMAL_HOST")
		if normalHost == "" {
			continue
		}
		missing := []string{}
		thermalHost := os.Getenv(prefix + "THERMAL_HOST")
		panTiltHost := os.Getenv(prefix + "PANTILT_HOST")
		zoomHost := os.Getenv(prefix + "ZOOM_HOST")
		if thermalHost == "" {
			missing = append(missing, "THERMAL_HOST")
		}
		if panTiltHost == "" {
			missing = append(missing, "PANTILT_HOST")
		}
		if zoomHost == "" {
			missing = append(missing, "ZOOM_HOST")
		}
		if len(missing) > 0 {
			log.Printf("DEVICE_%d: missing %v, skipping", i, missing)
			continue
		}

		port := envInt(prefix+"PORT", 80)
		rtspPort := envInt(prefix+"RTSP_PORT", 554)
		channel := envInt(prefix+"CHANNEL", 1)
		thermalMainStream := os.Getenv(prefix+"THERMAL_MAIN_STREAM") == "true"

		mkCam := func(host string, mainStream bool) *Camera {
			return &Camera{
				Host: host, Port: port, RTSPPort: rtspPort,
				Channel: channel, Username: username, Password: password,
				UseMainStream: mainStream,
			}
		}

		dev := &Device{
			ID:          i,
			Name:        getEnv(prefix+"NAME", "Device "+strconv.Itoa(i)),
			NormalCam:   mkCam(normalHost, false),
			ThermalCam:  mkCam(thermalHost, thermalMainStream),
			PanTiltCtrl: mkCam(panTiltHost, false),
			ZoomCtrl:    mkCam(zoomHost, false),
		}
		devices = append(devices, dev)
		log.Printf("Loaded device %d: %s (normal=%s thermal=%s pantilt=%s zoom=%s)",
			dev.ID, dev.Name, normalHost, thermalHost, panTiltHost, zoomHost)
	}
	return devices
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
