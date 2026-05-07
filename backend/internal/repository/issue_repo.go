package repository

import (
	"strconv"
	"time"

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

func (r *IssueRepo) List(projectID uint64, zentaoStatus, testStatus, branch, keyword, assignee string, offset, limit int) ([]model.Issue, int64, error) {
	query := r.db.Model(&model.Issue{})

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
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
		// 支持通过本地ID或禅道ID精确搜索，或通过标题/描述模糊搜索
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			query = query.Where("id = ? OR zentao_id = ?", uint64(id), id)
		} else {
			query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	if assignee != "" {
		query = query.Where("assignee = ?", assignee)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var issues []model.Issue
	if err := query.Offset(offset).Limit(limit).Order("resolved_at IS NULL ASC, resolved_at DESC, id DESC").Find(&issues).Error; err != nil {
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

// FindByZentaoIssueID 根据禅道问题单ID查找issue
func (r *IssueRepo) FindByZentaoIssueID(zentaoIssueID int) (*model.Issue, error) {
	var issue model.Issue
	err := r.db.Where("zentao_id = ?", zentaoIssueID).First(&issue).Error
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

// UpdateDevFlowSubmitTime 更新研发流水线提交时间并设置状态为待升级
func (r *IssueRepo) UpdateDevFlowSubmitTime(id uint64, submitTime time.Time, devTaskID string) error {
	return r.db.Model(&model.Issue{}).Where("id = ?", id).Updates(map[string]interface{}{
		"dev_flow_submit_time": submitTime,
		"dev_task_id":          devTaskID,
		"test_status":          model.TestStatusPendingUpgrade,
	}).Error
}

// FindPendingUpgradeBeforeTime 查询提交时间在指定时间之前且状态为待升级的问题单
func (r *IssueRepo) FindPendingUpgradeBeforeTime(ciStartTime time.Time) ([]model.Issue, error) {
	var issues []model.Issue
	err := r.db.Where("dev_flow_submit_time < ? AND dev_flow_submit_time IS NOT NULL AND test_status = ?",
		ciStartTime, model.TestStatusPendingUpgrade).
		Order("dev_flow_submit_time ASC").
		Find(&issues).Error
	return issues, err
}

// BatchUpdateTestStatus 批量更新测试状态
func (r *IssueRepo) BatchUpdateTestStatus(ids []uint64, newStatus string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&model.Issue{}).Where("id IN ?", ids).Update("test_status", newStatus).Error
}

// MarkMissingAsClosed 将API未返回的issue标记为closed
func (r *IssueRepo) MarkMissingAsClosed(projectID uint64, keepZentaoIDs []int, syncedAt time.Time) ([]model.Issue, error) {
	query := r.db.Model(&model.Issue{}).Where("project_id = ?", projectID)
	if len(keepZentaoIDs) > 0 {
		query = query.Where("zentao_id NOT IN ?", keepZentaoIDs)
	}
	// 只更新状态不是closed的issue
	query = query.Where("zentao_status != ?", "closed")

	var issues []model.Issue
	if err := query.Find(&issues).Error; err != nil {
		return nil, err
	}

	if len(issues) == 0 {
		return issues, nil
	}

	// 批量更新为closed
	ids := make([]uint64, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}

	err := r.db.Model(&model.Issue{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"zentao_status": "closed",
		"synced_at":     syncedAt,
	}).Error

	return issues, err
}
