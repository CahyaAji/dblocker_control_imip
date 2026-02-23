package main

import (
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := getEnv("MQTT_BROKER", "tcp://localhost:1883")
	username := getEnv("MQTT_USERNAME", "DBL0KER")
	password := getEnv("MQTT_PASSWORD", "4;1Yf,)`")

	// Configure Client Options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("retained-remover")
	opts.SetUsername(username)
	opts.SetPassword(password)

	// Create and Connect the Client
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connected to Broker")

	topicsToRemove := []string{
		"dbl/250002/c",
		"dbl/250001/cmd",
	}

	for _, topic := range topicsToRemove {
		// Publish empty payload (nil) with retained=true to clear the message
		token := client.Publish(topic, 1, true, []byte{})
		token.Wait()
		if token.Error() != nil {
			fmt.Printf("Error clearing %s: %v\n", topic, token.Error())
		} else {
			fmt.Printf("Cleared retained message for: %s\n", topic)
		}
	}

	time.Sleep(500 * time.Millisecond)
	client.Disconnect(250)
	fmt.Println("Done.")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
