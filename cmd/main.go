package main

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database"
	"dblocker_control/internal/infrastructure/database/repository"
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

	mqttBroker := "tcp://127.0.0.1:1883"

	mqttClient, err := mqtt.New(mqttBroker, "dblocker-server")
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Close()

	dblockerRepo := repository.NewDBlockerRepository(db)

	bridgeSvc, err := service.NewBridgeService(mqttClient, dblockerRepo)
	if err != nil {
		log.Fatalf("Failed to initialize bridge service: %v", err)
	}

	bridgeHandler := handlerhttp.NewBridgeHandler(bridgeSvc)

	r := gin.Default()
	//! remove this in production, only for development
	r.Use(cors.Default())

	route.RegisterHTTPRoutes(r, db, mqttClient, bridgeHandler)

	log.Printf("Starting dblocker server on :8080 (bridging %s)", bridgeSvc.Topic())
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
