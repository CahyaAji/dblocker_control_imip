package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const maxRecordDuration = 10 * time.Minute

// activeRecording holds state for an in-progress recording.
type activeRecording struct {
	cam       string // "normal" | "thermal"
	detect    bool   // recording the YOLO-annotated stream
	filename  string // basename only, e.g. "device1_normal_20260508_153045.mp4"
	startedAt time.Time
	cancel    context.CancelFunc
}

// frameSource is implemented by both StreamBroadcaster and DetectionBroadcaster.
// It's the minimal interface the recorder needs to subscribe to a camera feed.
type frameSource interface {
	Subscribe() (uint64, <-chan []byte)
	Unsubscribe(uint64)
}

// recorderMu and recorder are added to Device in camera.go.
// They are declared here alongside the logic that uses them.

// RecordStart begins recording the chosen camera for this device.
// duration is capped at maxRecordDuration; pass 0 to use the maximum.
// If detect is true and cam == "normal", the recording captures the YOLO-
// annotated MJPEG stream (the detector must be attached for this to work).
// Files are written to recordDir.
func (d *Device) RecordStart(cam string, detect bool, duration time.Duration, recordDir string) error {
	var camera *Camera
	switch cam {
	case "normal":
		camera = d.NormalCam
	case "thermal":
		camera = d.ThermalCam
	default:
		return fmt.Errorf("unknown cam %q; use normal or thermal", cam)
	}

	// Detection is only meaningful for the normal cam and requires the detector.
	var src frameSource
	if detect {
		if cam != "normal" {
			return fmt.Errorf("detect recording is only available for the normal camera")
		}
		bc := camera.GetDetectBroadcaster()
		if bc == nil {
			return fmt.Errorf("detector not configured (set DETECT_MODEL_PATH)")
		}
		src = bc
	} else {
		src = camera.GetBroadcaster()
	}

	if duration <= 0 || duration > maxRecordDuration {
		duration = maxRecordDuration
	}

	// Verify the record directory exists and is writable before locking.
	// We do NOT use MkdirAll here on purpose: if the path is a mount point
	// that isn't mounted, MkdirAll would silently create the folder on the
	// root filesystem instead of failing. We want an explicit error.
	if err := checkRecordDir(recordDir); err != nil {
		return err
	}

	d.recorderMu.Lock()
	defer d.recorderMu.Unlock()
	if d.recorder != nil {
		return fmt.Errorf("already recording %s camera", d.recorder.cam)
	}

	ts := time.Now()
	suffix := ""
	if detect {
		suffix = "_detect"
	}
	filename := fmt.Sprintf("device%d_%s%s_%s.mp4", d.ID, cam, suffix, ts.Format("20060102_150405"))
	outPath := filepath.Join(recordDir, filename)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	d.recorder = &activeRecording{
		cam:       cam,
		detect:    detect,
		filename:  filename,
		startedAt: ts,
		cancel:    cancel,
	}

	go d.runRecorder(ctx, cancel, src, cam, outPath)
	return nil
}

func (d *Device) runRecorder(ctx context.Context, cancel context.CancelFunc, src frameSource, cam, outPath string) {
	defer cancel()
	defer func() {
		d.recorderMu.Lock()
		d.recorder = nil
		d.recorderMu.Unlock()
	}()

	subID, frames := src.Subscribe()
	defer src.Unsubscribe(subID)

	// Do NOT use exec.CommandContext — it sends SIGKILL which corrupts the MP4.
	// Instead we close stdin to let FFmpeg finalize gracefully.
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-framerate", "15",
		"-i", "pipe:0",
		"-vcodec", "libx264",
		"-preset", "fast",
		"-crf", "23",
		outPath,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[recorder device%d %s] stdin pipe: %v", d.ID, cam, err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[recorder device%d %s] ffmpeg start: %v", d.ID, cam, err)
		return
	}

	log.Printf("[recorder device%d %s] started → %s", d.ID, cam, filepath.Base(outPath))

loop:
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				break loop
			}
			if _, err := stdin.Write(frame); err != nil {
				log.Printf("[recorder device%d %s] write: %v", d.ID, cam, err)
				break loop
			}
		case <-ctx.Done():
			break loop
		}
	}

	// Closing stdin sends EOF to FFmpeg, which lets it finalize the MP4 moov atom cleanly.
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		log.Printf("[recorder device%d %s] ffmpeg exit: %v", d.ID, cam, err)
	} else {
		log.Printf("[recorder device%d %s] saved → %s", d.ID, cam, filepath.Base(outPath))
	}
}

// RecordStop cancels the active recording for this device.
func (d *Device) RecordStop() error {
	d.recorderMu.Lock()
	defer d.recorderMu.Unlock()
	if d.recorder == nil {
		return fmt.Errorf("not recording")
	}
	d.recorder.cancel()
	return nil
}

// RecordStatus returns a snapshot of the current recording state.
func (d *Device) RecordStatus() (recording bool, cam, filename string, detect bool, startedAt time.Time) {
	d.recorderMu.Lock()
	defer d.recorderMu.Unlock()
	if d.recorder == nil {
		return false, "", "", false, time.Time{}
	}
	return true, d.recorder.cam, d.recorder.filename, d.recorder.detect, d.recorder.startedAt
}

// checkRecordDir verifies that dir exists, is a directory, and is writable.
// It does NOT create the directory — if the path is a missing mount point,
// that is a configuration error and should be reported clearly.
func checkRecordDir(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("record directory %q does not exist — check RECORD_DIR_HOST in your .env and ensure the disk is mounted", dir)
	}
	if err != nil {
		return fmt.Errorf("record directory %q is not accessible: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("record path %q exists but is not a directory", dir)
	}
	// Write-test: try creating and immediately removing a temp file.
	tmp, err := os.CreateTemp(dir, ".rec-writetest-*")
	if err != nil {
		return fmt.Errorf("record directory %q is not writable: %w", dir, err)
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return nil
}
