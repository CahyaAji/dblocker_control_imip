package models

type DBlockerConfig struct {
	SignalGPS  bool `json:"signal_gps" default:"false"`
	SignalCtrl bool `json:"signal_ctrl" default:"false"`
}

type DBlocker struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	Name       string           `json:"name" binding:"required"`
	SerialNumb string           `json:"serial_numb" binding:"required"`
	IP         string           `json:"ip" binding:"required"`
	Lat        float64          `json:"latitude" binding:"required"`
	Lng        float64          `json:"longitude" binding:"required"`
	Desc       string           `json:"desc"`
	AngleStart int              `json:"angle_start" default:"0"`
	Config     []DBlockerConfig `gorm:"serializer:json;type:jsonb" json:"config"`
}

type DBlockerConfigUpdate struct {
	ID     uint              `json:"id" binding:"required"`
	Config [6]DBlockerConfig `gorm:"serializer:json;type:jsonb" json:"config"`
}
