package models

import "time"

type DroneDetector struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	Name     string    `json:"name" binding:"required"`
	Lat      float64   `json:"latitude" binding:"required"`
	Lng      float64   `json:"longitude" binding:"required"`
	Host     string    `json:"host" binding:"required"`
	Port     int       `json:"port" binding:"required"`
	Status   string    `gorm:"default:offline" json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

type DroneEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DetectorID uint      `json:"detector_id"`
	Detector   string    `json:"detector"`
	UniqueID   string    `json:"unique_id"`
	TargetName string    `json:"target_name"`
	DroneLat   float64   `json:"drone_lat"`
	DroneLng   float64   `json:"drone_lng"`
	DroneAlt   int       `json:"drone_alt"`
	Heading    int       `json:"heading"`
	Distance   int       `json:"distance"`
	Speed      float64   `json:"speed"`
	Frequency  float64   `json:"frequency"`
	Confidence uint8     `json:"confidence"`
	RemoteLat  float64   `json:"remote_lat"`
	RemoteLng  float64   `json:"remote_lng"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
