package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"os/exec"
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
			log.Printf("[stream %s] ffmpeg pipe error: %v", c.Host, err)
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Printf("[stream %s] ffmpeg stderr pipe error: %v", c.Host, err)
			return
		}
		if err := cmd.Start(); err != nil {
			log.Printf("[stream %s] ffmpeg start error: %v", c.Host, err)
			return
		}
		// Forward ffmpeg warnings/errors to the server log.
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					log.Printf("[ffmpeg %s] %s", c.Host, string(buf[:n]))
				}
				if err != nil {
					if err != io.EOF {
						log.Printf("[stream %s] ffmpeg stderr read: %v", c.Host, err)
					}
					return
				}
			}
		}()
		defer cmd.Wait()

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
						// incomplete frame — keep buf from start, wait for more data
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
						return
					default:
						// drop frame if consumer is too slow
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return frames
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
