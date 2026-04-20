package main

import (
	handlerhttp "dblocker_control/internal/handler/http"
	"dblocker_control/internal/infrastructure/database"
	"dblocker_control/internal/infrastructure/database/repository"
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"dblocker_control/internal/route"
	"dblocker_control/internal/service"
	"fmt"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	db, err := database.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	mqttBroker := getEnv("MQTT_BROKER", "tcp://mosquitto:1883")
	appPort := getEnv("APP_PORT", "8080")

	mqttClient, err := mqtt.New(mqttBroker, "dblocker-server")
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Close()

	dblockerRepo := repository.NewDBlockerRepository(db)

	monitorSvc := service.NewCurrentMonitorService()
	fanControlSvc := service.NewFanControlService(mqttClient)

	bridgeSvc, err := service.NewBridgeService(mqttClient, dblockerRepo, monitorSvc, fanControlSvc)
	if err != nil {
		log.Fatalf("Failed to initialize bridge service: %v", err)
	}

	// Wire auto-off callback: turn off all sectors when temperature exceeds limit
	fanControlSvc.SetAutoOffCallback(func(serial string) {
		dblockers, err := dblockerRepo.FindAll()
		if err != nil {
			log.Printf("auto-off: failed to find dblockers: %v", err)
			return
		}
		for _, d := range dblockers {
			if d.SerialNumb == serial {
				allOff := make([]models.DBlockerConfig, 6)
				if err := dblockerRepo.UpdateConfig(d.ID, allOff); err != nil {
					log.Printf("auto-off: failed to update config for %s: %v", serial, err)
					return
				}
				fanM, fanS := fanControlSvc.FanState(serial, allOff)
				bitmask, err := service.DBlockerConfigToBitmask(allOff, fanM, fanS)
				if err != nil {
					log.Printf("auto-off: failed to build bitmask for %s: %v", serial, err)
					return
				}
				topic := fmt.Sprintf("dbl/%s/cmd", serial)
				payload := []byte{byte(bitmask >> 8), byte(bitmask)}
				if err := mqttClient.Publish(topic, 1, true, payload); err != nil {
					log.Printf("auto-off: failed to publish for %s: %v", serial, err)
				} else {
					log.Printf("auto-off: turned off %s (%s) due to high temperature", d.Name, serial)
				}
				return
			}
		}
		log.Printf("auto-off: device with serial %s not found", serial)
	})

	bridgeHandler := handlerhttp.NewBridgeHandler(bridgeSvc)

	// Auth setup
	userRepo := repository.NewUserRepository(db)
	jwtSecret := getEnv("JWT_SECRET", "")
	apiKey := getEnv("API_KEY", "")
	authSvc := service.NewAuthService(userRepo, jwtSecret, apiKey)

	adminUser := getEnv("ADMIN_USERNAME", "admin")
	adminPass := getEnv("ADMIN_PASSWORD", "admin")
	if err := authSvc.SeedAdmin(adminUser, adminPass); err != nil {
		log.Printf("warn: failed to seed admin user: %v", err)
	}

	r := gin.Default()
	//! remove this in production, only for development
	r.Use(cors.Default())

	route.RegisterHTTPRoutes(r, db, mqttClient, bridgeHandler, bridgeSvc, authSvc)

	log.Printf("Starting dblocker server on :%s (bridging %s)", appPort, bridgeSvc.Topic())
	if err := r.Run(":" + appPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
