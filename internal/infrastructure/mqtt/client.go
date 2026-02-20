package mqtt

import (
	"fmt"
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Client interface {
	Publish(topic string, qos byte, retained bool, payload any) error
	Subscribe(topic string, qos byte, handler Handler) error
	Unsubscribe(topics ...string) error
	Close()
}

// Message wraps the subset of MQTT message fields that handlers typically need.
type Message struct {
	Topic   string
	Payload []byte
}

// Handler is a simplified message handler signature decoupled from the paho dependency.
type Handler func(Message)

type mqttClient struct {
	pahoClient paho.Client
}

func New(broker string, clientID string) (Client, error) {
	opts := paho.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetAutoReconnect(true)

	opts.OnConnect = func(c paho.Client) {
		log.Println("Connected to MQTT broker")

	}
	opts.OnConnectionLost = func(c paho.Client, err error) {
		log.Printf("Connection lost: %v", err)
	}

	opts.OnReconnecting = func(c paho.Client, options *paho.ClientOptions) {
		log.Println("Attempting to reconnect to MQTT broker...")
	}

	client := paho.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect error: %w", token.Error())
	}

	return &mqttClient{pahoClient: client}, nil

}

func (m *mqttClient) Publish(topic string, qos byte, retained bool, payload any) error {
	token := m.pahoClient.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (m *mqttClient) Subscribe(topic string, qos byte, handler Handler) error {
	var wrapped paho.MessageHandler
	if handler != nil {
		wrapped = func(_ paho.Client, msg paho.Message) {
			handler(Message{Topic: msg.Topic(), Payload: msg.Payload()})
		}
	}

	token := m.pahoClient.Subscribe(topic, qos, wrapped)
	token.Wait()
	return token.Error()
}

func (m *mqttClient) Unsubscribe(topics ...string) error {
	token := m.pahoClient.Unsubscribe(topics...)
	token.Wait()
	return token.Error()
}

func (m *mqttClient) Close() {
	m.pahoClient.Disconnect(250)
}
