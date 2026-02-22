package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	cfg := parseFlags()

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.broker).
		SetClientID(cfg.clientID).
		SetCleanSession(true)

	if cfg.username != "" {
		opts.SetUsername(cfg.username)
		opts.SetPassword(cfg.password)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("MQTT connect failed: %v", token.Error())
	}
	defer client.Disconnect(250)

	for i := 1; i <= cfg.repeat; i++ {
		payload := fmt.Sprintf(cfg.messageTemplate, i)
		token := client.Publish(cfg.topic, cfg.qos, cfg.retain, payload)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Fatalf("publish failed: %v", err)
		}
		log.Printf("[%d/%d] Published to %s", i, cfg.repeat, cfg.topic)
		if i < cfg.repeat {
			time.Sleep(cfg.interval)
		}
	}

	log.Println("All messages published")
}

type publisherConfig struct {
	broker          string
	topic           string
	message         string
	messageTemplate string
	repeat          int
	interval        time.Duration
	qos             byte
	retain          bool
	clientID        string
	username        string
	password        string
}

func parseFlags() publisherConfig {
	cfg := publisherConfig{}
	flag.StringVar(&cfg.broker, "broker", "tcp://localhost:1883", "MQTT broker URI (e.g. tcp://localhost:1883)")
	flag.StringVar(&cfg.topic, "topic", "test/coba", "Topic to publish to")
	flag.StringVar(&cfg.message, "message", "hello from publisher", "Message payload. If repeating and no format verbs are present, a counter suffix is added.")
	flag.IntVar(&cfg.repeat, "repeat", 1, "Number of messages to publish")
	flag.DurationVar(&cfg.interval, "interval", time.Second, "Delay between repeated publishes")
	var qos int
	flag.IntVar(&qos, "qos", 0, "Publish QoS level (0, 1, 2)")
	cfg.qos = byte(qos)
	flag.BoolVar(&cfg.retain, "retain", false, "Mark message as retained")
	flag.StringVar(&cfg.clientID, "client-id", "dblocker-publisher", "MQTT client identifier")
	flag.StringVar(&cfg.username, "username", "", "Username for authenticated brokers")
	flag.StringVar(&cfg.password, "password", "", "Password for authenticated brokers")
	flag.Parse()

	if cfg.qos > 2 {
		log.Fatalf("invalid QoS %d (must be 0, 1, or 2)", cfg.qos)
	}
	if cfg.repeat < 1 {
		log.Fatalf("repeat must be >= 1")
	}

	cfg.messageTemplate = cfg.message
	if cfg.repeat > 1 && !containsCounterVerb(cfg.message) {
		cfg.messageTemplate = cfg.message + " #%d"
	}

	return cfg
}

func containsCounterVerb(msg string) bool {
	for i := 0; i < len(msg)-1; i++ {
		if msg[i] == '%' {
			next := msg[i+1]
			if next == 'd' || next == 's' || next == 'v' {
				return true
			}
		}
	}
	return false
}
