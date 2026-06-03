package repository

import (
	"dblocker_control/internal/models"
	"time"

	"gorm.io/gorm"
)

type DBlockerRepository struct {
	DB *gorm.DB
}

func NewDBlockerRepository(db *gorm.DB) *DBlockerRepository {
	return &DBlockerRepository{DB: db}
}

func (r *DBlockerRepository) Create(dblocker *models.DBlocker) error {
	return r.DB.Create(dblocker).Error
}

func (r *DBlockerRepository) FindAll() ([]models.DBlocker, error) {
	var dblockers []models.DBlocker
	err := r.DB.Find(&dblockers).Error
	return dblockers, err
}

func (r *DBlockerRepository) FindByID(id uint) (*models.DBlocker, error) {
	var dblocker models.DBlocker
	err := r.DB.First(&dblocker, id).Error
	return &dblocker, err
}

func (r *DBlockerRepository) Delete(id uint) error {
	return r.DB.Delete(&models.DBlocker{}, id).Error
}

func (r *DBlockerRepository) Update(dblocker *models.DBlocker) error {
	return r.DB.Save(dblocker).Error
}

func (r *DBlockerRepository) UpdateConfig(id uint, config []models.DBlockerConfig) error {
	return r.DB.Model(&models.DBlocker{ID: id}).Select("Config").Updates(models.DBlocker{Config: config}).Error
}

func (r *DBlockerRepository) UpdatePresetConfig(id uint, presetConfig []models.DBlockerConfig) error {
	return r.DB.Model(&models.DBlocker{ID: id}).Select("PresetConfig").Updates(models.DBlocker{PresetConfig: presetConfig}).Error
}

func (r *DBlockerRepository) UpdateDefaultConfig(id uint, defaultConfig []models.DBlockerConfig) error {
	return r.DB.Model(&models.DBlocker{ID: id}).Select("DefaultConfig").Updates(models.DBlocker{DefaultConfig: defaultConfig}).Error
}

func (r *DBlockerRepository) UpdateLastOnlineAt(serial string, t time.Time) error {
	return r.DB.Model(&models.DBlocker{}).
		Where("serial_numb = ?", serial).
		Update("last_online_at", t).Error
}

// FindBySerial returns the dblocker with the given serial number.
func (r *DBlockerRepository) FindBySerial(serial string) (*models.DBlocker, error) {
	var d models.DBlocker
	err := r.DB.Where("serial_numb = ?", serial).First(&d).Error
	return &d, err
}
