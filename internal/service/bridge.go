package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"fmt"
	"log"
	"strings"
	"sync"
)

// liveChannelBuf is the buffer size for each SSE subscriber's live message channel.
// Retained /sta snapshot bypasses this channel entirely, so the size is
// independent of device count and only needs to absorb short bursts of live messages.
const liveChannelBuf = 64

// BridgeService subscribes to MQTT and fans messages to subscribers.
type BridgeService struct {
	mu          sync.RWMutex // guards topics and lastByTopic
	refreshMu   sync.Mutex   // serializes concurrent RefreshTopics calls
	client      mqtt.Client
	reader      DBlockerReader
	topics      []string
	lastByTopic map[string]mqtt.Message // only /sta topics; one entry per serial
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
		lastByTopic: make(map[string]mqtt.Message),
		broadcaster: newBroadcaster(),
	}

	if registrar, ok := client.(mqtt.OnConnectRegistrar); ok {
		registrar.AddOnConnectHandler(func() {
			if err := br.ResubscribeTrackedTopics(); err != nil {
				log.Printf("Failed to resubscribe bridge topics after MQTT reconnect: %v", err)
			}
		})
	}

	if err := br.RefreshTopics(); err != nil {
		return nil, err
	}

	return br, nil
}

func (b *BridgeService) subscribeTopics(topics []string) error {
	for _, topic := range topics {
		if err := b.client.Subscribe(topic, 0, func(msg mqtt.Message) {
			b.broadcast(msg)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (b *BridgeService) RefreshTopics() error {
	// Serialize concurrent calls (e.g. simultaneous create+delete requests)
	// to prevent interleaved subscribe/unsubscribe mutations.
	b.refreshMu.Lock()
	defer b.refreshMu.Unlock()

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

		deviceTopics := []string{
			fmt.Sprintf("dbl/%s/rpt", serial),
			fmt.Sprintf("dbl/%s/sta", serial),
		}

		for _, topic := range deviceTopics {
			if _, exists := nextSet[topic]; exists {
				continue
			}

			nextSet[topic] = struct{}{}
			nextTopics = append(nextTopics, topic)
		}
	}

	// Compute diffs and clean stale cache under the lock, but do NOT call
	// client.Subscribe/Unsubscribe while holding it: paho can deliver a
	// retained message synchronously inside Subscribe(), which calls
	// broadcast() → tries to re-acquire b.mu → deadlock.
	b.mu.Lock()
	currentSet := make(map[string]struct{}, len(b.topics))
	for _, topic := range b.topics {
		currentSet[topic] = struct{}{}
	}

	toSubscribe := make([]string, 0)
	for _, topic := range nextTopics {
		if _, exists := currentSet[topic]; !exists {
			toSubscribe = append(toSubscribe, topic)
		}
	}

	toUnsubscribe := make([]string, 0)
	for _, topic := range b.topics {
		if _, exists := nextSet[topic]; !exists {
			toUnsubscribe = append(toUnsubscribe, topic)
			if strings.HasSuffix(topic, "/sta") {
				delete(b.lastByTopic, topic) // evict stale /sta cache entry
			}
		}
	}
	b.mu.Unlock()

	if err := b.subscribeTopics(toSubscribe); err != nil {
		return err
	}

	if len(toUnsubscribe) > 0 {
		if err := b.client.Unsubscribe(toUnsubscribe...); err != nil {
			return err
		}
	}

	b.mu.Lock()
	b.topics = nextTopics
	b.mu.Unlock()
	return nil
}

func (b *BridgeService) ResubscribeTrackedTopics() error {
	b.refreshMu.Lock()
	defer b.refreshMu.Unlock()

	b.mu.RLock()
	topics := append([]string(nil), b.topics...)
	b.mu.RUnlock()

	if len(topics) == 0 {
		return nil
	}

	if err := b.subscribeTopics(topics); err != nil {
		return err
	}

	log.Printf("Resubscribed %d MQTT bridge topics after reconnect", len(topics))
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

// Snapshot returns the last known payload for all /sta topics.
// Replayed to new SSE clients so retained status messages are immediately visible.
func (b *BridgeService) Snapshot() []mqtt.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	msgs := make([]mqtt.Message, 0, len(b.lastByTopic))
	for _, msg := range b.lastByTopic {
		msgs = append(msgs, msg)
	}
	return msgs
}

// Subscribe returns a buffered channel that receives live MQTT messages.
// Snapshot replay bypasses this channel, so the buffer only needs to
// absorb short live-message bursts; see liveChannelBuf.
func (b *BridgeService) Subscribe() chan mqtt.Message {
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()
	ch := make(chan mqtt.Message, liveChannelBuf)
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
	if strings.HasSuffix(msg.Topic, "/sta") {
		b.mu.Lock()
		b.lastByTopic[msg.Topic] = msg
		b.mu.Unlock()
	}

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
