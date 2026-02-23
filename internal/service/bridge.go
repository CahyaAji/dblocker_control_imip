package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"fmt"
	"strings"
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

func NewBridgeService(client mqtt.Client, dblockerReader DBlockerReader) (*BridgeService, error) {
	dblockers, err := dblockerReader.FindAll()
	if err != nil {
		return nil, err
	}

	br := &BridgeService{topics: make([]string, 0, len(dblockers)), broadcaster: newBroadcaster()}
	seenTopics := make(map[string]struct{})

	for _, dblocker := range dblockers {
		serial := strings.TrimSpace(dblocker.SerialNumb)
		if serial == "" {
			continue
		}

		topic := fmt.Sprintf("dbl/%s/rpt", serial)
		if _, exists := seenTopics[topic]; exists {
			continue
		}

		if err := client.Subscribe(topic, 0, func(msg mqtt.Message) {
			br.broadcast(msg)
		}); err != nil {
			return nil, err
		}

		seenTopics[topic] = struct{}{}
		br.topics = append(br.topics, topic)
	}

	if len(br.topics) == 0 {
		return nil, fmt.Errorf("no dblocker serial numbers found to subscribe")
	}

	return br, nil
}

func (b *BridgeService) Topic() string {
	return strings.Join(b.topics, ",")
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
