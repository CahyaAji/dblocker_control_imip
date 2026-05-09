package repository

import (
	"dblocker_control/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppSettingRepository struct {
	DB *gorm.DB
}

func NewAppSettingRepository(db *gorm.DB) *AppSettingRepository {
	return &AppSettingRepository{DB: db}
}

// Get returns the value for a key, or defaultVal if not found.
func (r *AppSettingRepository) Get(key, defaultVal string) string {
	var s models.AppSetting
	if err := r.DB.First(&s, "key = ?", key).Error; err != nil {
		return defaultVal
	}
	return s.Value
}

// Set upserts a key-value pair.
func (r *AppSettingRepository) Set(key, value string) error {
	s := models.AppSetting{Key: key, Value: value}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&s).Error
}
