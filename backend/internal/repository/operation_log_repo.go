package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type OperationLogRepo struct {
	db *gorm.DB
}

func NewOperationLogRepo() *OperationLogRepo {
	return &OperationLogRepo{db: DB}
}

func (r *OperationLogRepo) Create(log *model.OperationLog) error {
	return r.db.Create(log).Error
}

func (r *OperationLogRepo) ListAuthLogs(username, action string, offset, limit int) ([]model.OperationLog, int64, error) {
	query := r.db.Model(&model.OperationLog{}).Where("module = ?", "auth")

	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.OperationLog
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
