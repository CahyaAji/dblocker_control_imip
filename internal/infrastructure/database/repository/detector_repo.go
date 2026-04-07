package repository

import (
	"dblocker_control/internal/models"
	"time"

	"gorm.io/gorm"
)

type DetectorRepository struct {
	DB *gorm.DB
}

func NewDetectorRepository(db *gorm.DB) *DetectorRepository {
	return &DetectorRepository{DB: db}
}

func (r *DetectorRepository) Create(d *models.DroneDetector) error {
	return r.DB.Create(d).Error
}

func (r *DetectorRepository) FindAll() ([]models.DroneDetector, error) {
	var detectors []models.DroneDetector
	err := r.DB.Find(&detectors).Error
	return detectors, err
}

func (r *DetectorRepository) FindByID(id uint) (*models.DroneDetector, error) {
	var d models.DroneDetector
	err := r.DB.First(&d, id).Error
	return &d, err
}

func (r *DetectorRepository) Update(d *models.DroneDetector) error {
	return r.DB.Save(d).Error
}

func (r *DetectorRepository) Delete(id uint) error {
	return r.DB.Delete(&models.DroneDetector{}, id).Error
}

func (r *DetectorRepository) UpdateStatus(id uint, status string) error {
	return r.DB.Model(&models.DroneDetector{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "last_seen": time.Now()}).Error
}

func (r *DetectorRepository) FindByHostPort(host string, port int) (*models.DroneDetector, error) {
	var d models.DroneDetector
	err := r.DB.Where("host = ? AND port = ?", host, port).First(&d).Error
	return &d, err
}

// --- Drone Events ---

type DroneEventRepository struct {
	DB *gorm.DB
}

func NewDroneEventRepository(db *gorm.DB) *DroneEventRepository {
	return &DroneEventRepository{DB: db}
}

func (r *DroneEventRepository) Create(e *models.DroneEvent) error {
	return r.DB.Create(e).Error
}

func (r *DroneEventRepository) FindLatestByTarget(targetName string) (*models.DroneEvent, error) {
	var ev models.DroneEvent
	err := r.DB.Where("target_name = ?", targetName).Order("created_at DESC").First(&ev).Error
	return &ev, err
}

func (r *DroneEventRepository) FindFiltered(from, to time.Time, detectorID uint, limit int) ([]models.DroneEvent, error) {
	var events []models.DroneEvent
	q := r.DB.Where("created_at >= ? AND created_at <= ?", from, to)
	if detectorID > 0 {
		q = q.Where("detector_id = ?", detectorID)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *DroneEventRepository) DeleteByDateRange(from, to time.Time) (int64, error) {
	result := r.DB.Where("created_at >= ? AND created_at <= ?", from, to).Delete(&models.DroneEvent{})
	return result.RowsAffected, result.Error
}
