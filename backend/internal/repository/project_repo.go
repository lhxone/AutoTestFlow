package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type ProjectMetricProject struct {
	ProjectID   uint64
	ProjectName string
}

type ProjectMetricCounts struct {
	ProjectID          uint64
	ClosedCount        int64
	AIResolvedCount    int64
	ProcessingCount    int64
	PendingReviewCount int64
}

type projectMetricCountRow struct {
	ProjectID uint64
	Count     int64
}

type ProjectRepo struct {
	db *gorm.DB
}

func NewProjectRepo() *ProjectRepo {
	return &ProjectRepo{db: DB}
}

func (r *ProjectRepo) Create(project *model.Project) error {
	return r.db.Create(project).Error
}

func (r *ProjectRepo) GetByID(id uint64) (*model.Project, error) {
	var p model.Project
	err := r.db.Preload("Owner").First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepo) Update(project *model.Project) error {
	return r.db.Save(project).Error
}

func (r *ProjectRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Project{}, id).Error
}

func (r *ProjectRepo) List(keyword string, status *int8, offset, limit int) ([]model.Project, int64, error) {
	query := r.db.Model(&model.Project{}).Preload("Owner")

	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var projects []model.Project
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// GetAllActive 获取所有启用的项目
func (r *ProjectRepo) GetAllActive() ([]model.Project, error) {
	var projects []model.Project
	err := r.db.Where("status = 1").Find(&projects).Error
	return projects, err
}

func (r *ProjectRepo) ListMetricProjects(projectID uint64, includeDisabled bool) ([]ProjectMetricProject, error) {
	query := r.db.Model(&model.Project{}).
		Select("id AS project_id, name AS project_name").
		Order("id ASC")

	if projectID > 0 {
		query = query.Where("id = ?", projectID)
	}
	if !includeDisabled {
		query = query.Where("status = ?", 1)
	}

	var projects []ProjectMetricProject
	if err := query.Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *ProjectRepo) GetMetricCounts(projectIDs []uint64) (map[uint64]ProjectMetricCounts, error) {
	result := make(map[uint64]ProjectMetricCounts, len(projectIDs))
	if len(projectIDs) == 0 {
		return result, nil
	}

	for _, projectID := range projectIDs {
		result[projectID] = ProjectMetricCounts{ProjectID: projectID}
	}

	merge := func(rows []projectMetricCountRow, apply func(*ProjectMetricCounts, int64)) {
		for _, row := range rows {
			counts := result[row.ProjectID]
			apply(&counts, row.Count)
			result[row.ProjectID] = counts
		}
	}

	var closedRows []projectMetricCountRow
	if err := r.db.Table("issue").
		Select("project_id, COUNT(*) AS count").
		Where("project_id IN ? AND zentao_status = ?", projectIDs, "closed").
		Group("project_id").
		Scan(&closedRows).Error; err != nil {
		return nil, err
	}
	merge(closedRows, func(counts *ProjectMetricCounts, count int64) {
		counts.ClosedCount = count
	})

	processingStatuses := []string{
		model.TestStatusPendingUpgrade,
		model.TestStatusPendingGenerate,
		model.TestStatusGenerating,
		model.TestStatusTesting,
		model.TestStatusInterventionNeeded,
		model.TestStatusInterventionInProgress,
		model.TestStatusError,
		model.TestStatusReviewRejected,
	}
	var processingRows []projectMetricCountRow
	if err := r.db.Table("issue").
		Select("project_id, COUNT(*) AS count").
		Where("project_id IN ? AND test_status IN ?", projectIDs, processingStatuses).
		Group("project_id").
		Scan(&processingRows).Error; err != nil {
		return nil, err
	}
	merge(processingRows, func(counts *ProjectMetricCounts, count int64) {
		counts.ProcessingCount = count
	})

	var pendingReviewRows []projectMetricCountRow
	if err := r.db.Table("review_task").
		Select("project_id, COUNT(DISTINCT issue_id) AS count").
		Where("project_id IN ? AND status = ?", projectIDs, model.ReviewStatusPending).
		Group("project_id").
		Scan(&pendingReviewRows).Error; err != nil {
		return nil, err
	}
	merge(pendingReviewRows, func(counts *ProjectMetricCounts, count int64) {
		counts.PendingReviewCount = count
	})

	var aiResolvedRows []projectMetricCountRow
	if err := r.db.Table("issue").
		Select("issue.project_id, COUNT(DISTINCT issue.id) AS count").
		Joins("LEFT JOIN review_task ON review_task.issue_id = issue.id AND review_task.status = ?", model.ReviewStatusApproved).
		Where("issue.project_id IN ? AND issue.zentao_status = ? AND (issue.test_status = ? OR review_task.id IS NOT NULL)",
			projectIDs, "closed", model.TestStatusReviewApproved).
		Group("issue.project_id").
		Scan(&aiResolvedRows).Error; err != nil {
		return nil, err
	}
	merge(aiResolvedRows, func(counts *ProjectMetricCounts, count int64) {
		counts.AIResolvedCount = count
	})

	return result, nil
}
