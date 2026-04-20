package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"fmt"
	"log"
	"strings"
	"sync"
)

// liveChannelBuf is the buffer size for each SSE subscriber's live /rpt channel.
const liveChannelBuf = 64

// staChannelBuf is the buffer size for each SSE subscriber's /sta channel.
// /sta messages are critical (status changes) and must never be dropped.
const staChannelBuf = 32

// BridgeService subscribes to MQTT and fans messages to subscribers.
type BridgeService struct {
	mu              sync.RWMutex // guards topics and lastByTopic
	refreshMu       sync.Mutex   // serializes concurrent RefreshTopics calls
	client          mqtt.Client
	reader          DBlockerReader
	topics          []string
	lastByTopic     map[string]mqtt.Message // only /sta topics; one entry per serial
	lastRptBySerial map[string]string       // latest /rpt payload per serial
	broadcaster     *broadcaster
	monitor         *CurrentMonitorService
	fanControl      *FanControlService
}

// Subscriber holds separate channels for /sta (priority) and /rpt (best-effort).
type Subscriber struct {
	StaCh  chan mqtt.Message // status changes — never dropped
	LiveCh chan mqtt.Message // /rpt sensor data — dropped if slow
}

type broadcaster struct {
	mu          sync.Mutex
	subscribers map[*Subscriber]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{subscribers: make(map[*Subscriber]struct{})}
}

type DBlockerReader interface {
	FindAll() ([]models.DBlocker, error)
}

// NewBridgeService wires the MQTT subscription to the broadcaster.

func NewBridgeService(client mqtt.Client, reader DBlockerReader, monitor *CurrentMonitorService, fanControl *FanControlService) (*BridgeService, error) {
	br := &BridgeService{
		client:          client,
		reader:          reader,
		topics:          make([]string, 0),
		lastByTopic:     make(map[string]mqtt.Message),
		lastRptBySerial: make(map[string]string),
		broadcaster:     newBroadcaster(),
		monitor:         monitor,
		fanControl:      fanControl,
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

	br.resetRetainedStatus()

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

		// Initialize fan control state from DB config
		if b.fanControl != nil {
			b.fanControl.InitDevice(serial, dblocker.Config)
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

// resetRetainedStatus publishes OFF retained for all tracked /sta topics
// to clear potentially stale retained messages from the MQTT broker.
// Connected devices will re-publish their real status shortly after.
func (b *BridgeService) resetRetainedStatus() {
	b.mu.RLock()
	topics := append([]string(nil), b.topics...)
	b.mu.RUnlock()

	count := 0
	for _, topic := range topics {
		if strings.HasSuffix(topic, "/sta") {
			if err := b.client.Publish(topic, 0, true, []byte("OFF")); err != nil {
				log.Printf("Failed to reset retained status for %s: %v", topic, err)
			} else {
				count++
			}
		}
	}
	if count > 0 {
		log.Printf("Reset %d retained /sta messages to OFF", count)
	}
}

func (b *BridgeService) Topic() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.topics) == 0 {
		return ""
	}

	return b.topics[0]
}

// Monitor returns the current monitor service.
func (b *BridgeService) Monitor() *CurrentMonitorService {
	return b.monitor
}

// FanControl returns the fan control service.
func (b *BridgeService) FanControl() *FanControlService {
	return b.fanControl
}

// LastRpt returns the latest /rpt payload for a serial, or empty string if none.
func (b *BridgeService) LastRpt(serial string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastRptBySerial[serial]
}

// SubscribeWithSnapshot atomically takes a /sta snapshot and creates subscriber
// channels, ensuring no /sta message can fall between snapshot and subscription.
func (b *BridgeService) SubscribeWithSnapshot() (*Subscriber, []mqtt.Message) {
	// Lock both mutexes: broadcaster.mu to register the subscriber,
	// and b.mu to read the snapshot — all before returning.
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()

	b.mu.RLock()
	snap := make([]mqtt.Message, 0, len(b.lastByTopic))
	for _, msg := range b.lastByTopic {
		snap = append(snap, msg)
	}
	b.mu.RUnlock()

	sub := &Subscriber{
		StaCh:  make(chan mqtt.Message, staChannelBuf),
		LiveCh: make(chan mqtt.Message, liveChannelBuf),
	}
	b.broadcaster.subscribers[sub] = struct{}{}
	return sub, snap
}

func (b *BridgeService) Unsubscribe(sub *Subscriber) {
	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()
	if _, ok := b.broadcaster.subscribers[sub]; ok {
		delete(b.broadcaster.subscribers, sub)
		close(sub.StaCh)
		close(sub.LiveCh)
	}
}

func (b *BridgeService) broadcast(msg mqtt.Message) {
	if strings.HasSuffix(msg.Topic, "/sta") {
		b.mu.Lock()
		b.lastByTopic[msg.Topic] = msg
		b.mu.Unlock()
	}

	// Feed /rpt messages to the current monitor
	if strings.HasSuffix(msg.Topic, "/rpt") && b.monitor != nil {
		// Extract serial from topic: dbl/{serial}/rpt
		parts := strings.SplitN(msg.Topic, "/", 3)
		if len(parts) == 3 {
			serial := parts[1]
			b.mu.Lock()
			b.lastRptBySerial[serial] = string(msg.Payload)
			b.mu.Unlock()
			b.monitor.HandleRpt(serial, string(msg.Payload))
			if b.fanControl != nil {
				b.fanControl.HandleTemperature(serial, string(msg.Payload))
			}
		}
	}

	b.broadcaster.mu.Lock()
	defer b.broadcaster.mu.Unlock()

	isSta := strings.HasSuffix(msg.Topic, "/sta")
	for sub := range b.broadcaster.subscribers {
		if isSta {
			// /sta messages are critical — block briefly to ensure delivery.
			select {
			case sub.StaCh <- msg:
			default:
				// Buffer full — should be rare. Log and skip to avoid blocking broadcast.
				log.Printf("warn: /sta channel full, dropping message for topic %s", msg.Topic)
			}
		} else {
			// /rpt messages are best-effort — drop if subscriber is slow.
			select {
			case sub.LiveCh <- msg:
			default:
			}
		}
	}
}
