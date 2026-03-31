package repository

import (
	"dblocker_control/internal/models"

	"gorm.io/gorm"
)

type ScheduleRepository struct {
	DB *gorm.DB
}

func NewScheduleRepository(db *gorm.DB) *ScheduleRepository {
	return &ScheduleRepository{DB: db}
}

func (r *ScheduleRepository) Create(schedule *models.Schedule) error {
	return r.DB.Create(schedule).Error
}

func (r *ScheduleRepository) FindAll() ([]models.Schedule, error) {
	var schedules []models.Schedule
	err := r.DB.Preload("DBlocker").Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepository) FindByID(id uint) (*models.Schedule, error) {
	var schedule models.Schedule
	err := r.DB.Preload("DBlocker").First(&schedule, id).Error
	return &schedule, err
}

func (r *ScheduleRepository) Update(schedule *models.Schedule) error {
	return r.DB.Save(schedule).Error
}

func (r *ScheduleRepository) Delete(id uint) error {
	return r.DB.Delete(&models.Schedule{}, id).Error
}

func (r *ScheduleRepository) FindEnabled() ([]models.Schedule, error) {
	var schedules []models.Schedule
	err := r.DB.Preload("DBlocker").Where("enabled = ?", true).Find(&schedules).Error
	return schedules, err
}
