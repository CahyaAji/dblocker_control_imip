package main

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	mqttBroker := "tcp://127.0.0.1:1883"

	mqttClient, err := mqtt.New(mqttBroker, "dblocker-server")
	if err != nil {
		log.Printf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Close()

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Println("Starting dblocker server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
