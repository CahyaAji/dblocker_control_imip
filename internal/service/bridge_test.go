package service

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"reflect"
	"sync"
	"testing"
)

type fakeMQTTClient struct {
	mu                sync.Mutex
	subscribedTopics  []string
	unsubscribedTopic []string
	onConnectHandlers []func()
}

func (f *fakeMQTTClient) Publish(topic string, qos byte, retained bool, payload any) error {
	return nil
}

func (f *fakeMQTTClient) Subscribe(topic string, qos byte, handler mqtt.Handler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribedTopics = append(f.subscribedTopics, topic)
	return nil
}

func (f *fakeMQTTClient) Unsubscribe(topics ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsubscribedTopic = append(f.unsubscribedTopic, topics...)
	return nil
}

func (f *fakeMQTTClient) Close() {}

func (f *fakeMQTTClient) AddOnConnectHandler(handler func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onConnectHandlers = append(f.onConnectHandlers, handler)
}

func (f *fakeMQTTClient) TriggerConnect() {
	f.mu.Lock()
	handlers := append([]func(){}, f.onConnectHandlers...)
	f.mu.Unlock()

	for _, handler := range handlers {
		handler()
	}
}

func (f *fakeMQTTClient) snapshotSubscribedTopics() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.subscribedTopics...)
}

type fakeDBlockerReader struct {
	dblockers []models.DBlocker
}

func (f fakeDBlockerReader) FindAll() ([]models.DBlocker, error) {
	return append([]models.DBlocker(nil), f.dblockers...), nil
}

func TestNewBridgeServiceResubscribesTrackedTopicsOnReconnect(t *testing.T) {
	client := &fakeMQTTClient{}
	reader := fakeDBlockerReader{dblockers: []models.DBlocker{{SerialNumb: "250001"}}}

	bridge, err := NewBridgeService(client, reader, NewCurrentMonitorService())
	if err != nil {
		t.Fatalf("NewBridgeService() error = %v", err)
	}

	initialTopics := client.snapshotSubscribedTopics()
	expectedTopics := []string{"dbl/250001/rpt", "dbl/250001/sta"}
	if !reflect.DeepEqual(initialTopics, expectedTopics) {
		t.Fatalf("initial subscriptions = %v, want %v", initialTopics, expectedTopics)
	}

	client.TriggerConnect()

	afterReconnectTopics := client.snapshotSubscribedTopics()
	wantAfterReconnect := []string{
		"dbl/250001/rpt",
		"dbl/250001/sta",
		"dbl/250001/rpt",
		"dbl/250001/sta",
	}
	if !reflect.DeepEqual(afterReconnectTopics, wantAfterReconnect) {
		t.Fatalf("subscriptions after reconnect = %v, want %v", afterReconnectTopics, wantAfterReconnect)
	}

	if bridge.Topic() != "dbl/250001/rpt" {
		t.Fatalf("Topic() = %q, want %q", bridge.Topic(), "dbl/250001/rpt")
	}
}
