package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IssueRepo struct {
	db *gorm.DB
}

func NewIssueRepo() *IssueRepo {
	return &IssueRepo{db: DB}
}

// Upsert 插入或更新问题单(按 zentao_id + project_id 判重)
func (r *IssueRepo) Upsert(issue *model.Issue) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "zentao_id"}, {Name: "project_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "description", "issue_type", "zentao_status",
			"severity", "priority", "reporter", "reporter_email",
			"assignee", "assignee_email", "branch",
			"resolved_at", "zentao_updated_at", "synced_at",
		}),
	}).Create(issue).Error
}

func (r *IssueRepo) GetByID(id uint64) (*model.Issue, error) {
	var issue model.Issue
	err := r.db.Preload("Project").First(&issue, id).Error
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

func (r *IssueRepo) List(projectID, projectSetID uint64, zentaoStatus, testStatus, branch, keyword, assignee string, offset, limit int) ([]model.Issue, int64, error) {
	query := r.db.Model(&model.Issue{})

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if projectSetID > 0 {
		query = query.Where("project_id IN (SELECT project_id FROM project WHERE project_set_id = ?)", projectSetID)
	}
	if zentaoStatus != "" {
		query = query.Where("zentao_status = ?", zentaoStatus)
	}
	if testStatus != "" {
		query = query.Where("test_status = ?", testStatus)
	}
	if branch != "" {
		query = query.Where("branch = ?", branch)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if assignee != "" {
		query = query.Where("assignee = ?", assignee)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var issues []model.Issue
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&issues).Error; err != nil {
		return nil, 0, err
	}

	return issues, total, nil
}

// GetResolvedPending 获取禅道"已解决"且测试状态为"pending"的问题单
func (r *IssueRepo) GetResolvedPending(projectID uint64) ([]model.Issue, error) {
	var issues []model.Issue
	err := r.db.Where("project_id = ? AND zentao_status = 'resolved' AND test_status = ?",
		projectID, model.TestStatusPending).Find(&issues).Error
	return issues, err
}

// UpdateTestStatus 更新测试状态
func (r *IssueRepo) UpdateTestStatus(id uint64, oldStatus, newStatus string) error {
	result := r.db.Model(&model.Issue{}).
		Where("id = ? AND test_status = ?", id, oldStatus).
		Update("test_status", newStatus)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// ForceUpdateTestStatus 强制更新测试状态(不校验旧状态)
func (r *IssueRepo) ForceUpdateTestStatus(id uint64, newStatus string) error {
	return r.db.Model(&model.Issue{}).Where("id = ?", id).Update("test_status", newStatus).Error
}

// CreateStatusLog 记录状态变更日志
func (r *IssueRepo) CreateStatusLog(log *model.IssueStatusLog) error {
	return r.db.Create(log).Error
}

func (r *IssueRepo) FindByProjectAndZentaoIDs(projectID uint64, zentaoIDs []int) ([]model.Issue, error) {
	if len(zentaoIDs) == 0 {
		return []model.Issue{}, nil
	}

	var issues []model.Issue
	err := r.db.Where("project_id = ? AND zentao_id IN ?", projectID, zentaoIDs).Find(&issues).Error
	return issues, err
}

func (r *IssueRepo) DeleteMissingByProject(projectID uint64, keepZentaoIDs []int) (int64, error) {
	query := r.db.Where("project_id = ?", projectID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}

	result := query.Delete(&model.Issue{})
	return result.RowsAffected, result.Error
}

func (r *IssueRepo) ListMissingByProject(projectID uint64, keepZentaoIDs []int) ([]model.Issue, error) {
	query := r.db.Where("project_id = ?", projectID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}

	var issues []model.Issue
	err := query.Order("id DESC").Find(&issues).Error
	return issues, err
}
