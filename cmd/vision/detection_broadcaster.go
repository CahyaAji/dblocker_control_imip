package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// DetectionBroadcaster runs YOLO inference on frames from a Camera's
// StreamBroadcaster and fans out the annotated JPEG frames to multiple
// HTTP viewers. A single inference goroutine is shared across all viewers.
//
// Lifecycle: lazily started on the first Subscribe; stopped automatically
// when the last subscriber leaves.
type DetectionBroadcaster struct {
	cam      *Camera
	detector *Detector

	// jpegQuality is used when re-encoding the annotated frame.
	jpegQuality int

	mu      sync.Mutex
	subs    map[uint64]chan []byte
	nextID  uint64
	running bool
	cancel  context.CancelFunc
}

func newDetectionBroadcaster(cam *Camera, detector *Detector, jpegQuality int) *DetectionBroadcaster {
	if jpegQuality <= 0 {
		jpegQuality = 75
	}
	return &DetectionBroadcaster{
		cam:         cam,
		detector:    detector,
		jpegQuality: jpegQuality,
		subs:        make(map[uint64]chan []byte),
	}
}

func (b *DetectionBroadcaster) Subscribe() (uint64, <-chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	ch := make(chan []byte, 4)
	b.subs[id] = ch
	if !b.running {
		b.startLocked()
	}
	return id, ch
}

func (b *DetectionBroadcaster) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.subs[id]
	if !ok {
		return
	}
	close(ch)
	delete(b.subs, id)
	if len(b.subs) == 0 && b.running {
		// Mark stopped synchronously so a new Subscribe arriving before the
		// run goroutine wakes up starts a fresh one instead of attaching to
		// the dying one (which would close the new subscriber's channel).
		b.running = false
		b.cancel()
		b.cancel = nil
	}
}

func (b *DetectionBroadcaster) startLocked() {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	go b.run(ctx)
}

func (b *DetectionBroadcaster) run(ctx context.Context) {
	subID, frames := b.cam.GetBroadcaster().Subscribe()
	defer b.cam.GetBroadcaster().Unsubscribe(subID)

	for {
		select {
		case <-ctx.Done():
			b.closeAllSubs()
			return
		case frame, ok := <-frames:
			if !ok {
				b.closeAllSubs()
				return
			}
			start := time.Now()
			dets, img, err := b.detector.Run(frame)
			if err != nil {
				log.Printf("[detect %s] inference error: %v", b.cam.Host, err)
				continue
			}
			out, err := AnnotateAndEncode(img, dets, b.jpegQuality)
			if err != nil {
				log.Printf("[detect %s] annotate error: %v", b.cam.Host, err)
				continue
			}
			if d := time.Since(start); d > 250*time.Millisecond {
				log.Printf("[detect %s] frame took %s (%d detections)", b.cam.Host, d, len(dets))
			}

			b.mu.Lock()
			for _, ch := range b.subs {
				select {
				case ch <- out:
				default: // slow viewer — drop frame
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *DetectionBroadcaster) closeAllSubs() {
	b.mu.Lock()
	b.running = false
	b.cancel = nil
	for id, ch := range b.subs {
		close(ch)
		delete(b.subs, id)
	}
	b.mu.Unlock()
}
