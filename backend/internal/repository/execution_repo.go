package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type ExecutionRepo struct {
	db *gorm.DB
}

func NewExecutionRepo() *ExecutionRepo {
	return &ExecutionRepo{db: DB}
}

func (r *ExecutionRepo) Create(exec *model.TestExecution) error {
	return r.db.Create(exec).Error
}

func (r *ExecutionRepo) GetByID(id uint64) (*model.TestExecution, error) {
	var e model.TestExecution
	return &e, r.db.First(&e, id).Error
}

func (r *ExecutionRepo) Update(exec *model.TestExecution) error {
	return r.db.Save(exec).Error
}

func (r *ExecutionRepo) List(projectID, taskID uint64, status string, offset, limit int) ([]model.TestExecution, int64, error) {
	query := r.db.Model(&model.TestExecution{})
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if taskID > 0 {
		query = query.Joins("JOIN test_execution_issue tei ON tei.execution_id = test_execution.id").
			Where("tei.test_task_id = ?", taskID).
			Distinct("test_execution.id")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var execs []model.TestExecution
	if err := query.Select("test_execution.*").Offset(offset).Limit(limit).Order("test_execution.id DESC").Find(&execs).Error; err != nil {
		return nil, 0, err
	}
	return execs, total, nil
}

// ManualIntervention
func (r *ExecutionRepo) CreateIntervention(mi *model.ManualIntervention) error {
	return r.db.Create(mi).Error
}

func (r *ExecutionRepo) ListInterventions(issueID uint64) ([]model.ManualIntervention, error) {
	var records []model.ManualIntervention
	err := r.db.Preload("Operator").Where("issue_id = ?", issueID).
		Order("id DESC").Find(&records).Error
	return records, err
}

// TestReport
func (r *ExecutionRepo) CreateReport(report *model.TestReport) error {
	return r.db.Create(report).Error
}

func (r *ExecutionRepo) GetReportByExecutionID(execID uint64) (*model.TestReport, error) {
	var report model.TestReport
	return &report, r.db.Where("execution_id = ?", execID).First(&report).Error
}

// NotificationLog
func (r *ExecutionRepo) CreateNotificationLog(log *model.NotificationLog) error {
	return r.db.Create(log).Error
}

// OperationLog
func (r *ExecutionRepo) CreateOperationLog(log *model.OperationLog) error {
	return r.db.Create(log).Error
}

// GitCommitLog
func (r *ExecutionRepo) CreateGitCommitLog(log *model.GitCommitLog) error {
	return r.db.Create(log).Error
}
