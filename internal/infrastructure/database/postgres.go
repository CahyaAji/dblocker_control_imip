package database

import (
	"dblocker_control/internal/models"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresDB() (*gorm.DB, error) {
	host := "127.0.0.1"
	user := "scm"
	password := "sdfKLJ0)imip"
	port := "5432"
	sslmode := "disable"
	dbname := "dblocker-db"
	timezone := "UTC"

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
