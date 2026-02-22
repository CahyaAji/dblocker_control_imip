package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"sync"
)

// BridgeService subscribes to MQTT and fans messages to subscribers.
type BridgeService struct {
	topic       string
	broadcaster *broadcaster
}

type broadcaster struct {
	mu          sync.Mutex
	subscribers map[chan mqtt.Message]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{subscribers: make(map[chan mqtt.Message]struct{})}
}

// NewBridgeService wires the MQTT subscription to the broadcaster.
func NewBridgeService(client mqtt.Client, topic string) (*BridgeService, error) {
	br := &BridgeService{topic: topic, broadcaster: newBroadcaster()}
	if err := client.Subscribe(topic, 0, func(msg mqtt.Message) {
		br.broadcast(msg)
	}); err != nil {
		return nil, err
	}
	return br, nil
}

func (b *BridgeService) Topic() string {
	return b.topic
}

func (b *BridgeService) Subscribe() chan mqtt.Message {
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()
	ch := make(chan mqtt.Message, 8)
	b.broadcaster.subscribers[ch] = struct{}{}
	return ch
}

func (b *BridgeService) Unsubscribe(ch chan mqtt.Message) {
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()
	if _, ok := b.broadcaster.subscribers[ch]; ok {
		delete(b.broadcaster.subscribers, ch)
		close(ch)
	}
}

func (b *BridgeService) broadcast(msg mqtt.Message) {
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()
	for ch := range b.broadcaster.subscribers {
		select {
		case ch <- msg:
		default:
			// Drop if subscriber is slow.
		}
	}
}
