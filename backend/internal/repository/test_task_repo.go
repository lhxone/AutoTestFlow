package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
)

type TestTaskRepo struct {
	db *gorm.DB
}

func NewTestTaskRepo() *TestTaskRepo {
	return &TestTaskRepo{db: DB}
}

func (r *TestTaskRepo) Create(task *model.TestTask) error {
	return r.db.Create(task).Error
}

func (r *TestTaskRepo) GetByID(id uint64) (*model.TestTask, error) {
	var t model.TestTask
	err := r.db.Preload("Issue").Preload("Project").First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TestTaskRepo) Update(task *model.TestTask) error {
	return r.db.Save(task).Error
}

func (r *TestTaskRepo) DeleteArtifactsByTaskID(taskID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var caseIDs []uint64
		if err := tx.Model(&model.TestCase{}).Where("task_id = ?", taskID).Pluck("id", &caseIDs).Error; err != nil {
			return err
		}
		if len(caseIDs) > 0 {
			if err := tx.Where("test_case_id IN ?", caseIDs).Delete(&model.TestCaseVersion{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("task_id = ?", taskID).Delete(&model.TestCase{}).Error; err != nil {
			return err
		}

		var scriptIDs []uint64
		if err := tx.Model(&model.TestScript{}).Where("task_id = ?", taskID).Pluck("id", &scriptIDs).Error; err != nil {
			return err
		}
		if len(scriptIDs) > 0 {
			if err := tx.Where("test_script_id IN ?", scriptIDs).Delete(&model.TestScriptVersion{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("task_id = ?", taskID).Delete(&model.TestScript{}).Error; err != nil {
			return err
		}

		if err := tx.Where("task_id = ?", taskID).Delete(&model.TestDocument{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *TestTaskRepo) List(projectID, issueID uint64, keyword, status string, offset, limit int) ([]model.TestTask, int64, error) {
	query := r.db.Model(&model.TestTask{}).
		Preload("Issue").
		Joins("LEFT JOIN issue ON issue.id = test_task.issue_id")

	if projectID > 0 {
		query = query.Where("test_task.project_id = ?", projectID)
	}
	if issueID > 0 {
		query = query.Where("test_task.issue_id = ?", issueID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("issue.title LIKE ?", like)
	}
	if status != "" {
		query = query.Where("test_task.status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []model.TestTask
	if err := query.Offset(offset).Limit(limit).Order("test_task.id DESC").Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// TestCase 相关
func (r *TestTaskRepo) CreateTestCase(tc *model.TestCase) error {
	return r.db.Create(tc).Error
}

func (r *TestTaskRepo) Delete(id uint64) error {
	return r.db.Delete(&model.TestTask{}, id).Error
}

func (r *TestTaskRepo) GetTestCasesByTaskID(taskID uint64) ([]model.TestCase, error) {
	var cases []model.TestCase
	err := r.db.Where("task_id = ?", taskID).Order("id ASC").Find(&cases).Error
	return cases, err
}

func (r *TestTaskRepo) GetTestCaseByID(id uint64) (*model.TestCase, error) {
	var tc model.TestCase
	return &tc, r.db.First(&tc, id).Error
}

func (r *TestTaskRepo) ListTestCases(projectID, issueID, taskID uint64, keyword, category, source, selfTestResult string, offset, limit int) ([]model.TestCase, int64, error) {
	query := r.db.Model(&model.TestCase{})

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if issueID > 0 {
		query = query.Where("issue_id = ?", issueID)
	}
	if taskID > 0 {
		query = query.Where("task_id = ?", taskID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR steps LIKE ? OR `expected` LIKE ?", like, like, like)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if selfTestResult != "" {
		query = query.Where("self_test_result = ?", selfTestResult)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var cases []model.TestCase
	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&cases).Error; err != nil {
		return nil, 0, err
	}
	return cases, total, nil
}

func (r *TestTaskRepo) UpdateTestCase(tc *model.TestCase) error {
	return r.db.Save(tc).Error
}

func (r *TestTaskRepo) CreateTestCaseVersion(v *model.TestCaseVersion) error {
	return r.db.Create(v).Error
}

// TestScript 相关
func (r *TestTaskRepo) CreateTestScript(ts *model.TestScript) error {
	return r.db.Create(ts).Error
}

func (r *TestTaskRepo) GetTestScriptsByTaskID(taskID uint64) ([]model.TestScript, error) {
	var scripts []model.TestScript
	err := r.db.Where("task_id = ?", taskID).Order("id ASC").Find(&scripts).Error
	return scripts, err
}

func (r *TestTaskRepo) GetTestScriptByID(id uint64) (*model.TestScript, error) {
	var ts model.TestScript
	return &ts, r.db.First(&ts, id).Error
}

func (r *TestTaskRepo) UpdateTestScript(ts *model.TestScript) error {
	return r.db.Save(ts).Error
}

func (r *TestTaskRepo) CreateTestScriptVersion(v *model.TestScriptVersion) error {
	return r.db.Create(v).Error
}

// TestDocument 相关
func (r *TestTaskRepo) CreateTestDocument(td *model.TestDocument) error {
	return r.db.Create(td).Error
}

func (r *TestTaskRepo) GetTestDocsByTaskID(taskID uint64) ([]model.TestDocument, error) {
	var docs []model.TestDocument
	err := r.db.Where("task_id = ?", taskID).Order("id ASC").Find(&docs).Error
	return docs, err
}
