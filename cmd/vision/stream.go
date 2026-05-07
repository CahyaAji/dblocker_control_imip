package main

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"time"
)

// FrameProcessor is an optional hook to process a JPEG frame before it is sent
// to the client (e.g. run YOLO inference and draw bounding boxes).
// Return the processed frame bytes. Return nil to drop the frame.
type FrameProcessor func(frame []byte) []byte

// StartMJPEGStream launches an FFmpeg subprocess that pulls the camera's
// RTSP stream (sub-stream by default, main stream if UseMainStream is set)
// and pushes decoded JPEG frames into the returned channel.
// The channel is closed when the stream ends or ctx is cancelled.
//
// To add YOLO (or any per-frame processing), wrap the output channel:
//
//	raw := cam.StartMJPEGStream(ctx)
//	processed := ProcessFrames(ctx, raw, myYOLOProcessor)
func (c *Camera) StartMJPEGStream(ctx context.Context) <-chan []byte {
	frames := make(chan []byte, 4)
	go func() {
		defer close(frames)

		rtspURL := c.RTSPSubStreamURL()
		if c.UseMainStream {
			rtspURL = c.RTSPMainStreamURL()
		}

		const (
			retryDelay    = 3 * time.Second
			maxRetryDelay = 30 * time.Second
		)
		delay := retryDelay

		for {
			if ctx.Err() != nil {
				return
			}

			if err := c.runFFmpeg(ctx, rtspURL, frames); err != nil && ctx.Err() == nil {
				log.Printf("[stream %s] ffmpeg exited: %v — retrying in %s", c.Host, err, delay)
			} else if ctx.Err() == nil {
				log.Printf("[stream %s] ffmpeg exited — retrying in %s", c.Host, delay)
			}

			if ctx.Err() != nil {
				return
			}

			select {
			case <-time.After(delay):
				// double the delay each failure, capped at maxRetryDelay
				delay *= 2
				if delay > maxRetryDelay {
					delay = maxRetryDelay
				}
			case <-ctx.Done():
				return
			}

			// reset delay on a successful reconnect (camera was reachable)
			delay = retryDelay
		}
	}()
	return frames
}

// runFFmpeg launches one FFmpeg process and forwards frames to the channel.
// Returns when FFmpeg exits or ctx is cancelled.
func (c *Camera) runFFmpeg(ctx context.Context, rtspURL string, frames chan<- []byte) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-loglevel", "warning",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "5",
		"-r", "15",
		"pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				log.Printf("[ffmpeg %s] %s", c.Host, string(buf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()

	// Scan the raw byte stream for complete JPEG frames.
	// JPEG starts with 0xFF 0xD8 and ends with 0xFF 0xD9.
	buf := make([]byte, 0, 512*1024)
	tmp := make([]byte, 65536)
	for {
		n, err := stdout.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				start := bytes.Index(buf, []byte{0xFF, 0xD8})
				if start < 0 {
					buf = buf[:0]
					break
				}
				end := bytes.Index(buf[start+2:], []byte{0xFF, 0xD9})
				if end < 0 {
					buf = buf[start:]
					break
				}
				end = start + 2 + end + 2
				frame := make([]byte, end-start)
				copy(frame, buf[start:end])
				buf = buf[end:]
				select {
				case frames <- frame:
				case <-ctx.Done():
					cmd.Wait()
					return nil
				default:
					// drop frame if consumer is too slow
				}
			}
		}
		if err != nil {
			break
		}
	}
	return cmd.Wait()
}

// ProcessFrames wraps a frame channel with a FrameProcessor.
// Use this to insert YOLO or any other per-frame processing into the pipeline.
// Frames for which the processor returns nil are dropped.
func ProcessFrames(ctx context.Context, in <-chan []byte, processor FrameProcessor) <-chan []byte {
	out := make(chan []byte, 4)
	go func() {
		defer close(out)
		for {
			select {
			case frame, ok := <-in:
				if !ok {
					return
				}
				result := processor(frame)
				if result == nil {
					continue
				}
				select {
				case out <- result:
				case <-ctx.Done():
					return
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
