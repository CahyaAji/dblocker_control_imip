package repository

import (
	"dblocker_control/internal/models"
	"time"

	"gorm.io/gorm"
)

type ActionLogRepository struct {
	DB *gorm.DB
}

func NewActionLogRepository(db *gorm.DB) *ActionLogRepository {
	return &ActionLogRepository{DB: db}
}

// ensureTable creates the yearly table if it doesn't exist yet.
func (r *ActionLogRepository) ensureTable(tableName string) error {
	return r.DB.Table(tableName).AutoMigrate(&models.ActionLog{})
}

func (r *ActionLogRepository) Create(log *models.ActionLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}
	table := models.ActionLogTableName(log.Timestamp)
	if err := r.ensureTable(table); err != nil {
		return err
	}
	return r.DB.Table(table).Create(log).Error
}

func (r *ActionLogRepository) FindByDateRange(from, to time.Time, limit, offset int) ([]models.ActionLog, int64, error) {
	// Determine which yearly tables to query
	startYear := from.Year()
	endYear := to.Year()

	var allLogs []models.ActionLog
	var total int64

	for y := endYear; y >= startYear; y-- {
		t := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		table := models.ActionLogTableName(t)
		if err := r.ensureTable(table); err != nil {
			continue
		}

		var count int64
		r.DB.Table(table).Where("timestamp >= ? AND timestamp <= ?", from, to).Count(&count)
		total += count

		var logs []models.ActionLog
		r.DB.Table(table).Where("timestamp >= ? AND timestamp <= ?", from, to).
			Order("timestamp DESC").Limit(limit - len(allLogs)).Offset(max(0, offset-int(total-count))).
			Find(&logs)
		allLogs = append(allLogs, logs...)

		if len(allLogs) >= limit {
			allLogs = allLogs[:limit]
			break
		}
	}

	return allLogs, total, nil
}

func (r *ActionLogRepository) Delete(id uint, year int) error {
	t := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	table := models.ActionLogTableName(t)
	if err := r.ensureTable(table); err != nil {
		return err
	}
	return r.DB.Table(table).Delete(&models.ActionLog{}, id).Error
}
