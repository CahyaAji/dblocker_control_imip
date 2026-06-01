package models

import "time"

// DroneWhitelist stores drones that should be detected/logged but never trigger blocker activation.
// Type must be "unique_id" or "target_name".
type DroneWhitelist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"not null" json:"type" binding:"required,oneof=unique_id target_name"`
	Value     string    `gorm:"not null;uniqueIndex:idx_whitelist_type_value" json:"value" binding:"required"`
	Note      string    `json:"note"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
