// go run send_control.go sleep      # Put to sleep
// go run send_control.go wake       # Wake without reset
// go run send_control.go wake_rst   # Wake with full reset
// go run send_control.go rst_slave  # Reset slave only

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ========================================
// CONFIGURATION
// ========================================
const (
	broker    = "tcp://10.88.81.16:1883"
	username  = "DBL0KER"
	password  = "4;1Yf,)`"
	clientID  = "dblocker-control-sender"
	serialNum = "250005" // Change per device
)

// ========================================

func sendCommand(cmd string) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("connect failed: %w", token.Error())
	}
	defer client.Disconnect(250)

	topic := fmt.Sprintf("dbl/%s/cmd", serialNum)
	payload := []byte(cmd)

	pubToken := client.Publish(topic, 1, false, payload)
	if !pubToken.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if pubToken.Error() != nil {
		return fmt.Errorf("publish failed: %w", pubToken.Error())
	}

	fmt.Printf("Sent %q to %s\n", cmd, topic)
	return nil
}

func printUsage() {
	fmt.Println("Usage: go run send_control.go <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  sleep     - Put controller to sleep (turns off all outputs)")
	fmt.Println("  wake      - Wake controller without reset")
	fmt.Println("  wake_rst  - Wake controller with full reset")
	fmt.Println("  rst_slave - Reset slave MCU only")
	fmt.Println()
	fmt.Printf("Target: %s (serial: %s)\n", broker, serialNum)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := strings.ToUpper(os.Args[1])

	var mqttCmd string
	switch cmd {
	case "SLEEP":
		mqttCmd = "SLEEP"
	case "WAKE":
		mqttCmd = "WAKE"
	case "WAKE_RST":
		mqttCmd = "WAKE_RST"
	case "RST_SLAVE":
		mqttCmd = "RST_SLAVE"
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err := sendCommand(mqttCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
