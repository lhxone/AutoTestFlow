package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type TaskEventLogRepo struct {
	db *gorm.DB
}

func NewTaskEventLogRepo() *TaskEventLogRepo {
	return &TaskEventLogRepo{db: DB}
}

func (r *TaskEventLogRepo) Create(log *model.TaskEventLog) error {
	return r.db.Create(log).Error
}

func (r *TaskEventLogRepo) ListByTaskID(taskID uint64) ([]model.TaskEventLog, error) {
	var logs []model.TaskEventLog
	err := r.db.Where("task_id = ?", taskID).Order("seq ASC, id ASC").Find(&logs).Error
	return logs, err
}
