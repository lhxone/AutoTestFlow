package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type IssueSyncLogRepo struct {
	db *gorm.DB
}

func NewIssueSyncLogRepo() *IssueSyncLogRepo {
	return &IssueSyncLogRepo{db: DB}
}

func (r *IssueSyncLogRepo) Create(log *model.IssueSyncLog) error {
	return r.db.Create(log).Error
}

func (r *IssueSyncLogRepo) UpdateResult(id uint64, status string, added, updated, deleted int, errMsg string) error {
	now := modelTimeNow()
	return r.db.Model(&model.IssueSyncLog{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        status,
			"added_count":   added,
			"updated_count": updated,
			"deleted_count": deleted,
			"error_message": errMsg,
			"completed_at":  &now,
		}).Error
}

func (r *IssueSyncLogRepo) BatchCreateDetails(details []model.IssueSyncLogDetail) error {
	if len(details) == 0 {
		return nil
	}
	return r.db.Create(&details).Error
}

func (r *IssueSyncLogRepo) GetLatestByProjectIDs(projectIDs []uint64) ([]model.IssueSyncLog, error) {
	if len(projectIDs) == 0 {
		return []model.IssueSyncLog{}, nil
	}

	var logs []model.IssueSyncLog
	err := r.db.Raw(`
		SELECT l.*
		FROM issue_sync_log l
		INNER JOIN (
			SELECT project_id, MAX(id) AS max_id
			FROM issue_sync_log
			WHERE project_id IN ?
			GROUP BY project_id
		) latest ON latest.max_id = l.id
		ORDER BY l.id DESC
	`, projectIDs).Scan(&logs).Error
	return logs, err
}

// GetLatestByProjectIDsAndType 获取指定类型的最新同步日志
func (r *IssueSyncLogRepo) GetLatestByProjectIDsAndType(projectIDs []uint64, syncType string) ([]model.IssueSyncLog, error) {
	if len(projectIDs) == 0 {
		return []model.IssueSyncLog{}, nil
	}

	var logs []model.IssueSyncLog
	err := r.db.Raw(`
		SELECT l.*
		FROM issue_sync_log l
		INNER JOIN (
			SELECT project_id, MAX(id) AS max_id
			FROM issue_sync_log
			WHERE project_id IN ? AND sync_type = ?
			GROUP BY project_id
		) latest ON latest.max_id = l.id
		ORDER BY l.id DESC
	`, projectIDs, syncType).Scan(&logs).Error
	return logs, err
}

func (r *IssueSyncLogRepo) ListByProject(projectID uint64, offset, limit int) ([]model.IssueSyncLog, int64, error) {
	query := r.db.Model(&model.IssueSyncLog{}).Where("project_id = ?", projectID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.IssueSyncLog
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ListByProjectAndType 按项目和类型获取同步日志
func (r *IssueSyncLogRepo) ListByProjectAndType(projectID uint64, syncType string, offset, limit int) ([]model.IssueSyncLog, int64, error) {
	query := r.db.Model(&model.IssueSyncLog{}).Where("project_id = ? AND sync_type = ?", projectID, syncType)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.IssueSyncLog
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *IssueSyncLogRepo) GetByProjectAndID(projectID, logID uint64) (*model.IssueSyncLog, error) {
	var log model.IssueSyncLog
	if err := r.db.Where("project_id = ? AND id = ?", projectID, logID).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *IssueSyncLogRepo) ListDetailsByLogID(logID uint64) ([]model.IssueSyncLogDetail, error) {
	var details []model.IssueSyncLogDetail
	err := r.db.Where("sync_log_id = ?", logID).Order("id DESC").Find(&details).Error
	return details, err
}

// ListDetailsByLogIDPaginated 分页获取采集详情
func (r *IssueSyncLogRepo) ListDetailsByLogIDPaginated(logID uint64, offset, limit int) ([]model.IssueSyncLogDetail, int64, error) {
	query := r.db.Model(&model.IssueSyncLogDetail{}).Where("sync_log_id = ?", logID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var details []model.IssueSyncLogDetail
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&details).Error; err != nil {
		return nil, 0, err
	}

	return details, total, nil
}

// ListAll 获取所有项目的采集记录（支持项目ID和类型筛选）
func (r *IssueSyncLogRepo) ListAll(projectID *uint64, syncType *string, offset, limit int) ([]model.IssueSyncLog, int64, error) {
	query := r.db.Model(&model.IssueSyncLog{})
	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}
	if syncType != nil && *syncType != "" {
		query = query.Where("sync_type = ?", *syncType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.IssueSyncLog
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByID 根据ID获取采集记录
func (r *IssueSyncLogRepo) GetByID(logID uint64) (*model.IssueSyncLog, error) {
	var log model.IssueSyncLog
	if err := r.db.First(&log, logID).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
