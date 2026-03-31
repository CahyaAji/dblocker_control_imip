package models

import "time"

type Schedule struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	DBlockerID uint             `gorm:"not null" json:"dblocker_id" binding:"required"`
	DBlocker   DBlocker         `gorm:"foreignKey:DBlockerID" json:"dblocker,omitempty"`
	Config     []DBlockerConfig `gorm:"serializer:json;type:jsonb" json:"config" binding:"required"`
	Time       string           `gorm:"not null" json:"time" binding:"required"`
	Timezone   string           `gorm:"not null;default:'+00:00'" json:"timezone"`
	CreatedBy  string           `gorm:"not null;default:''" json:"created_by"`
	Enabled    bool             `gorm:"default:true" json:"enabled"`
	CreatedAt  time.Time        `json:"created_at"`
}

type CreateScheduleRequest struct {
	DBlockerID uint             `json:"dblocker_id" binding:"required"`
	Config     []DBlockerConfig `json:"config" binding:"required"`
	Time       string           `json:"time" binding:"required"`
	Timezone   string           `json:"timezone" binding:"required"`
}
