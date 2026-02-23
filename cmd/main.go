package main

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/route"
	"dblocker_control/internal/service"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	db, err := database.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	_ = db // Currently not used, but can be passed to services if needed

	mqttBroker := "tcp://127.0.0.1:1883"
	topic := "test/coba"

	mqttClient, err := mqtt.New(mqttBroker, "dblocker-server")
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Close()

	bridgeSvc, err := service.NewBridgeService(mqttClient, topic)
	if err != nil {
		log.Fatalf("Failed to subscribe to %s: %v", topic, err)
	}

	bridgeHandler := handlerhttp.NewBridgeHandler(bridgeSvc)

	r := gin.Default()
	//! remove this in production, only for development
	r.Use(cors.Default())

	route.RegisterHTTPRoutes(r, bridgeHandler)

	log.Printf("Starting dblocker server on :8080 (bridging %s)", topic)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
