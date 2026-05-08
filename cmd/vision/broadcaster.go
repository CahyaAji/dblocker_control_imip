package main

import (
	"context"
	"sync"
)

// StreamBroadcaster manages one Camera stream (via StartMJPEGStream) and fans out
// decoded JPEG frames to multiple concurrent subscribers (HTTP viewers, recorders, etc.).
//
// The internal FFmpeg process starts on the first Subscribe call and stops automatically
// when the last subscriber calls Unsubscribe.
type StreamBroadcaster struct {
	cam *Camera

	mu      sync.Mutex
	subs    map[uint64]chan []byte
	nextID  uint64
	running bool
	cancel  context.CancelFunc
}

func newStreamBroadcaster(cam *Camera) *StreamBroadcaster {
	return &StreamBroadcaster{
		cam:  cam,
		subs: make(map[uint64]chan []byte),
	}
}

// Subscribe registers a new consumer. Returns an id and a read-only frame channel.
// The channel is closed when Unsubscribe is called or the stream permanently fails.
func (b *StreamBroadcaster) Subscribe() (uint64, <-chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	ch := make(chan []byte, 8)
	b.subs[id] = ch
	if !b.running {
		b.startLocked()
	}
	return id, ch
}

// Unsubscribe removes the consumer. If it was the last subscriber, the underlying
// FFmpeg process is stopped.
func (b *StreamBroadcaster) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.subs[id]
	if !ok {
		return
	}
	close(ch)
	delete(b.subs, id)
	if len(b.subs) == 0 && b.running {
		b.cancel()
		// b.running is cleared by the run goroutine when it exits.
	}
}

// startLocked starts the broadcast goroutine. Must be called with b.mu held.
func (b *StreamBroadcaster) startLocked() {
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	go b.run(ctx)
}

func (b *StreamBroadcaster) run(ctx context.Context) {
	frames := b.cam.StartMJPEGStream(ctx)
	for frame := range frames {
		b.mu.Lock()
		for _, ch := range b.subs {
			select {
			case ch <- frame:
			default: // subscriber is slow; drop frame rather than blocking the broadcaster
			}
		}
		b.mu.Unlock()
	}

	// Stream ended (context cancelled or permanent FFmpeg failure).
	// Close all remaining subscriber channels so consumers can detect the end.
	b.mu.Lock()
	b.running = false
	for id, ch := range b.subs {
		close(ch)
		delete(b.subs, id)
	}
	b.mu.Unlock()
}
