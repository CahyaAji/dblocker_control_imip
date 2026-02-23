package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// decodeDBlockerMask decodes a 14-bit mask into a readable form.
// Bit order must match the MCU mapping used in DBlockerConfigToBitmask.
func decodeDBlockerMask(mask uint16) string {
	get := func(bit int) bool {
		return (mask & (1 << bit)) != 0
	}

	type ch struct {
		gps  bool
		ctrl bool
	}

	channels := make([]ch, 6)
	channels[0] = ch{gps: get(0), ctrl: get(1)}
	channels[1] = ch{gps: get(2), ctrl: get(3)}
	channels[2] = ch{gps: get(4), ctrl: get(5)}
	fanMaster := get(6)
	channels[3] = ch{gps: get(7), ctrl: get(8)}
	channels[4] = ch{gps: get(9), ctrl: get(10)}
	channels[5] = ch{gps: get(11), ctrl: get(12)}
	fanSlave := get(13)

	var b strings.Builder
	fmt.Fprintf(&b, "DBlocker mask=%d (0x%04X)\n", mask, mask)
	for i, c := range channels {
		fmt.Fprintf(&b, "  CH%d: GPS=%t, CTRL=%t\n", i+1, c.gps, c.ctrl)
	}
	fmt.Fprintf(&b, "  FanMaster=%t, FanSlave=%t", fanMaster, fanSlave)
	return b.String()
}

func main() {
	// 1. Configuration
	broker := "tcp://localhost:1883"
	username := "DBL0KER"
	password := "4;1Yf,)`"
	topic := "#"
	clientID := "fedora-laptop-client"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)

	// 2. Define the Message Handler
	var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		payload := msg.Payload()
		t := msg.Topic()

		// Decode only command topics with exact 2-byte payload (bitmask format).
		// This avoids decoding string commands like "SLEEP".
		if strings.HasSuffix(t, "/cmd") && len(payload) == 2 {
			mask := uint16(payload[0])<<8 | uint16(payload[1])
			fmt.Printf("📩 Received [%s]: raw=%v\n%s\n", t, payload, decodeDBlockerMask(mask))
			return
		}

		// Default: just print the payload as best-effort string
		fmt.Printf("📩 Received [%s]: %s (raw=%v)\n", t, string(payload), payload)
	}

	opts.SetDefaultPublishHandler(messageHandler)

	// 3. Connection Callbacks
	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("✅ Connected to MQTT broker!")
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		fmt.Printf("❌ Connection lost: %v\n", err)
	}

	// 4. Create and Connect Client
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("🔥 Error connecting: %v\n", token.Error())
		os.Exit(1)
	}

	// 5. Subscribe to Topic
	if token := client.Subscribe(topic, 1, nil); token.Wait() && token.Error() != nil {
		fmt.Printf("🔥 Error subscribing: %v\n", token.Error())
		os.Exit(1)
	}

	fmt.Printf("🛰️  Listening on MQTT topic: %s\n", topic)

	// Keep the program running until you press Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n👋 Disconnecting...")
	client.Disconnect(250)
	time.Sleep(1 * time.Second)
}
