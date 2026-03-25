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
	mu          sync.RWMutex
	client      mqtt.Client
	reader      DBlockerReader
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

func NewBridgeService(client mqtt.Client, reader DBlockerReader) (*BridgeService, error) {
	br := &BridgeService{
		client:      client,
		reader:      reader,
		topics:      make([]string, 0),
		broadcaster: newBroadcaster(),
	}

	if err := br.RefreshTopics(); err != nil {
		return nil, err
	}

	return br, nil
}

func (b *BridgeService) RefreshTopics() error {
	dblockers, err := b.reader.FindAll()
	if err != nil {
		return err
	}

	nextTopics := make([]string, 0, len(dblockers))
	nextSet := make(map[string]struct{}, len(dblockers))
	for _, dblocker := range dblockers {
		serial := strings.TrimSpace(dblocker.SerialNumb)
		if serial == "" {
			continue
		}

		topic := fmt.Sprintf("dbl/%s/rpt", serial)
		if _, exists := nextSet[topic]; exists {
			continue
		}

		nextSet[topic] = struct{}{}
		nextTopics = append(nextTopics, topic)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	currentSet := make(map[string]struct{}, len(b.topics))
	for _, topic := range b.topics {
		currentSet[topic] = struct{}{}
	}

	for _, topic := range nextTopics {
		if _, exists := currentSet[topic]; exists {
			continue
		}

		if err := b.client.Subscribe(topic, 0, func(msg mqtt.Message) {
			b.broadcast(msg)
		}); err != nil {
			return err
		}
	}

	toUnsubscribe := make([]string, 0)
	for _, topic := range b.topics {
		if _, exists := nextSet[topic]; exists {
			continue
		}
		toUnsubscribe = append(toUnsubscribe, topic)
	}

	if len(toUnsubscribe) > 0 {
		if err := b.client.Unsubscribe(toUnsubscribe...); err != nil {
			return err
		}
	}

	b.topics = nextTopics
	return nil
}

func (b *BridgeService) Topic() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

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
