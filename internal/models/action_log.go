package models

import (
	"fmt"
	"time"
)

type ActionLog struct {
	ID           uint             `gorm:"primaryKey" json:"id"`
	Timestamp    time.Time        `gorm:"not null;index" json:"timestamp"`
	Username     string           `gorm:"not null" json:"username"`
	Action       string           `gorm:"not null" json:"action"`
	DBlockerID   uint             `gorm:"not null" json:"dblocker_id"`
	DBlockerName string           `gorm:"not null;default:''" json:"dblocker_name"`
	Config       []DBlockerConfig `gorm:"serializer:json;type:jsonb" json:"config"`
}

// ActionLogTableName returns the yearly table name, e.g. "action_logs_2026".
func ActionLogTableName(t time.Time) string {
	return fmt.Sprintf("action_logs_%d", t.UTC().Year())
}
