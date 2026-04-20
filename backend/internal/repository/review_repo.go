package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type ReviewRepo struct {
	db *gorm.DB
}

func NewReviewRepo() *ReviewRepo {
	return &ReviewRepo{db: DB}
}

func (r *ReviewRepo) Create(task *model.ReviewTask) error {
	return r.db.Create(task).Error
}

func (r *ReviewRepo) GetByID(id uint64) (*model.ReviewTask, error) {
	var rt model.ReviewTask
	err := r.db.Preload("TestTask").Preload("Issue").Preload("Reviewer").First(&rt, id).Error
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *ReviewRepo) GetLatestByTestTaskID(testTaskID uint64) (*model.ReviewTask, error) {
	var rt model.ReviewTask
	err := r.db.Preload("TestTask").Preload("Issue").Preload("Reviewer").
		Where("test_task_id = ?", testTaskID).
		Order("id DESC").
		First(&rt).Error
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *ReviewRepo) Update(task *model.ReviewTask) error {
	return r.db.Model(&model.ReviewTask{}).
		Where("id = ?", task.ID).
		Updates(map[string]interface{}{
			"status":      task.Status,
			"reviewer_id": task.ReviewerID,
			"reviewed_at": task.ReviewedAt,
			"review_note": task.ReviewNote,
			"updated_at":  task.UpdatedAt,
		}).Error
}

func (r *ReviewRepo) List(projectID, taskID uint64, status string, reviewerID uint64, offset, limit int) ([]model.ReviewTask, int64, error) {
	query := r.db.Model(&model.ReviewTask{}).
		Preload("Issue").Preload("Reviewer")

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if taskID > 0 {
		query = query.Where("test_task_id = ?", taskID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if reviewerID > 0 {
		query = query.Where("reviewer_id = ?", reviewerID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []model.ReviewTask
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// CreateRecord 创建审核记录
func (r *ReviewRepo) CreateRecord(record *model.ReviewRecord) error {
	return r.db.Create(record).Error
}

// GetRecordsByTaskID 获取审核记录
func (r *ReviewRepo) GetRecordsByTaskID(taskID uint64) ([]model.ReviewRecord, error) {
	var records []model.ReviewRecord
	err := r.db.Preload("Reviewer").Where("review_task_id = ?", taskID).
		Order("id ASC").Find(&records).Error
	return records, err
}
