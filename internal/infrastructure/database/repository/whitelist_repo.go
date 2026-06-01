package repository

import (
	"dblocker_control/internal/models"

	"gorm.io/gorm"
)

type WhitelistRepository struct {
	DB *gorm.DB
}

func NewWhitelistRepository(db *gorm.DB) *WhitelistRepository {
	return &WhitelistRepository{DB: db}
}

func (r *WhitelistRepository) FindAll() ([]models.DroneWhitelist, error) {
	var entries []models.DroneWhitelist
	err := r.DB.Order("created_at desc").Find(&entries).Error
	return entries, err
}

func (r *WhitelistRepository) Create(entry *models.DroneWhitelist) error {
	return r.DB.Create(entry).Error
}

func (r *WhitelistRepository) Delete(id uint) error {
	return r.DB.Delete(&models.DroneWhitelist{}, id).Error
}
