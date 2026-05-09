package models

// AppSetting stores key-value configuration that persists across restarts.
type AppSetting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"not null;default:''" json:"value"`
}
