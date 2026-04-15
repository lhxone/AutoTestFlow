package service

import (
	"errors"
	"fmt"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type InterventionService struct {
	testTaskRepo  *repository.TestTaskRepo
	issueRepo     *repository.IssueRepo
	executionRepo *repository.ExecutionRepo
	logger        *zap.Logger
}

func NewInterventionService(logger *zap.Logger) *InterventionService {
	return &InterventionService{
		testTaskRepo:  repository.NewTestTaskRepo(),
		issueRepo:     repository.NewIssueRepo(),
		executionRepo: repository.NewExecutionRepo(),
		logger:        logger,
	}
}

// UpdateTestCase 人工修改测试用例
func (s *InterventionService) UpdateTestCase(caseID, operatorID uint64, req *dto.UpdateTestCaseRequest) error {
	tc, err := s.testTaskRepo.GetTestCaseByID(caseID)
	if err != nil {
		return errors.New("测试用例不存在")
	}

	// 保存旧版本
	oldVersion := &model.TestCaseVersion{
		TestCaseID:   tc.ID,
		Version:      tc.CurrentVersion,
		Title:        tc.Title,
		Precondition: tc.Precondition,
		Steps:        tc.Steps,
		Expected:     tc.Expected,
		Source:       tc.Source,
		ChangeNote:   "修改前版本备份",
		ChangedBy:    &operatorID,
	}
	// 忽略重复版本错误(可能已存在)
	_ = s.testTaskRepo.CreateTestCaseVersion(oldVersion)

	// 更新测试用例
	beforeSnapshot := fmt.Sprintf("Title: %s\nSteps: %s\nExpected: %s", tc.Title, tc.Steps, tc.Expected)

	if req.Title != "" {
		tc.Title = req.Title
	}
	if req.Precondition != "" {
		tc.Precondition = req.Precondition
	}
	if req.Steps != "" {
		tc.Steps = req.Steps
	}
	if req.Expected != "" {
		tc.Expected = req.Expected
	}
	tc.CurrentVersion++
	tc.Source = "manual"

	if err := s.testTaskRepo.UpdateTestCase(tc); err != nil {
		return err
	}

	// 保存新版本
	newVersion := &model.TestCaseVersion{
		TestCaseID:   tc.ID,
		Version:      tc.CurrentVersion,
		Title:        tc.Title,
		Precondition: tc.Precondition,
		Steps:        tc.Steps,
		Expected:     tc.Expected,
		Source:       "manual",
		ChangeNote:   req.ChangeNote,
		ChangedBy:    &operatorID,
	}
	_ = s.testTaskRepo.CreateTestCaseVersion(newVersion)

	afterSnapshot := fmt.Sprintf("Title: %s\nSteps: %s\nExpected: %s", tc.Title, tc.Steps, tc.Expected)

	// 记录人工介入
	intervention := &model.ManualIntervention{
		IssueID:          tc.IssueID,
		ProjectID:        tc.ProjectID,
		OperatorID:       operatorID,
		InterventionType: "modify_case",
		Description:      req.ChangeNote,
		BeforeSnapshot:   beforeSnapshot,
		AfterSnapshot:    afterSnapshot,
		Status:           "completed",
	}
	_ = s.executionRepo.CreateIntervention(intervention)

	// 更新问题单状态为"人工修复中"
	_ = s.issueRepo.ForceUpdateTestStatus(tc.IssueID, model.TestStatusInterventionInProgress)

	return nil
}

// UpdateTestScript 人工修改测试脚本
func (s *InterventionService) UpdateTestScript(scriptID, operatorID uint64, req *dto.UpdateTestScriptRequest) error {
	ts, err := s.testTaskRepo.GetTestScriptByID(scriptID)
	if err != nil {
		return errors.New("测试脚本不存在")
	}

	beforeSnapshot := ts.FileContent

	// 保存旧版本
	_ = s.testTaskRepo.CreateTestScriptVersion(&model.TestScriptVersion{
		TestScriptID: ts.ID,
		Version:      ts.CurrentVersion,
		FileContent:  ts.FileContent,
		Source:       ts.Source,
		ChangeNote:   "修改前版本备份",
		ChangedBy:    &operatorID,
	})

	// 更新脚本
	ts.FileContent = req.FileContent
	ts.CurrentVersion++
	ts.Source = "manual"

	if err := s.testTaskRepo.UpdateTestScript(ts); err != nil {
		return err
	}

	// 保存新版本
	_ = s.testTaskRepo.CreateTestScriptVersion(&model.TestScriptVersion{
		TestScriptID: ts.ID,
		Version:      ts.CurrentVersion,
		FileContent:  ts.FileContent,
		Source:       "manual",
		ChangeNote:   req.ChangeNote,
		ChangedBy:    &operatorID,
	})

	// 记录人工介入
	_ = s.executionRepo.CreateIntervention(&model.ManualIntervention{
		IssueID:          ts.IssueID,
		ProjectID:        ts.ProjectID,
		OperatorID:       operatorID,
		InterventionType: "modify_script",
		Description:      req.ChangeNote,
		BeforeSnapshot:   beforeSnapshot,
		AfterSnapshot:    ts.FileContent,
		Status:           "completed",
	})

	_ = s.issueRepo.ForceUpdateTestStatus(ts.IssueID, model.TestStatusInterventionInProgress)

	return nil
}

// GetInterventionHistory 获取问题单的人工介入记录
func (s *InterventionService) GetInterventionHistory(issueID uint64) ([]model.ManualIntervention, error) {
	return s.executionRepo.ListInterventions(issueID)
}
