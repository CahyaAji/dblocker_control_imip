package database

import (
	"dblocker_control/internal/models"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresDB() (*gorm.DB, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	user := getEnv("DB_USER", "scm")
	password := getEnv("DB_PASSWORD", "mysecretpassword")
	port := getEnv("DB_PORT", "5432")
	sslmode := getEnv("DB_SSLMODE", "disable")
	dbname := getEnv("DB_NAME", "dblocker-db")
	timezone := getEnv("DB_TIMEZONE", "UTC")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s", host, user, password, dbname, port, sslmode, timezone)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&models.DBlocker{})
	if err != nil {
		return nil, err
	}

	fmt.Println("Connected to Postgres successfully!")
	return db, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
