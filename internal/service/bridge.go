package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"sync"
)

// BridgeService subscribes to MQTT and fans messages to subscribers.
type BridgeService struct {
	topics      []string
	broadcaster *broadcaster
}

type broadcaster struct {
	mu          sync.Mutex
	subscribers map[chan mqtt.Message]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{subscribers: make(map[chan mqtt.Message]struct{})}
}

type DBlockerReader interface {
	FindAll() ([]models.DBlocker, error)
}

// NewBridgeService wires the MQTT subscription to the broadcaster.

func NewBridgeService(client mqtt.Client, _ DBlockerReader) (*BridgeService, error) {
	br := &BridgeService{topics: make([]string, 0, 1), broadcaster: newBroadcaster()}
	topic := "dbl/+/rpt"

	if err := client.Subscribe(topic, 0, func(msg mqtt.Message) {
		br.broadcast(msg)
	}); err != nil {
		return nil, err
	}

	br.topics = append(br.topics, topic)
	return br, nil
}

func (b *BridgeService) Topic() string {
	if len(b.topics) == 0 {
		return ""
	}

	return b.topics[0]
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
