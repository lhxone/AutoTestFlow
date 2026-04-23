package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

// ZentaoTestCaseGenService 禅道用例生成测试脚本服务（完全独立，不复用 GentestService）
type ZentaoTestCaseGenService struct {
	tcRepo       *repository.ZentaoTestCaseRepo
	projectRepo  *repository.ProjectRepo
	agentRepo    *repository.AgentRepo
	testTaskRepo *repository.TestTaskRepo
	einoRuntime  *EinoGenTestRuntime
	eventHub     *TaskEventHub
	logger       *zap.Logger
}

func NewZentaoTestCaseGenService(logger *zap.Logger) *ZentaoTestCaseGenService {
	return &ZentaoTestCaseGenService{
		tcRepo:       repository.NewZentaoTestCaseRepo(),
		projectRepo:  repository.NewProjectRepo(),
		agentRepo:    repository.NewAgentRepo(),
		testTaskRepo: repository.NewTestTaskRepo(),
		einoRuntime:  NewEinoGenTestRuntime(logger),
		eventHub:     DefaultTaskEventHub,
		logger:       logger,
	}
}

// GenerateScriptInput AI生成脚本的输入
type GenerateScriptInput struct {
	TestCaseID    uint64 `json:"test_case_id"`
	TestCaseTitle string `json:"test_case_title"`
	Precondition  string `json:"precondition"`
	Steps         string `json:"steps"`
	Expected      string `json:"expected"`
	Type          string `json:"type"`
	Priority      int8   `json:"priority"`
	ProjectID     uint64 `json:"project_id"`
	ProjectName   string `json:"project_name"`
	FuncDocPath   string `json:"func_doc_path"`
	DesignDocPath string `json:"design_doc_path"`
	DBDocPath     string `json:"db_doc_path"`
	TestDocPath   string `json:"test_doc_path"`
}

// GenerateScript 根据禅道用例生成测试脚本
func (s *ZentaoTestCaseGenService) GenerateScript(testCaseID uint64, agentID *uint64, createdBy *uint64) (*model.TestTask, error) {
	tc, err := s.tcRepo.GetByID(testCaseID)
	if err != nil {
		return nil, fmt.Errorf("用例不存在: %w", err)
	}

	project, err := s.projectRepo.GetByID(tc.ProductID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusGenerating)

	resolvedAgentID, err := s.resolveAgentID(agentID)
	if err != nil {
		s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusSynced)
		return nil, err
	}

	var resolvedAgent *model.Agent
	if resolvedAgentID != nil {
		resolvedAgent, err = s.agentRepo.GetByID(*resolvedAgentID)
		if err != nil {
			s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusSynced)
			return nil, fmt.Errorf("Agent不存在: %w", err)
		}
	}

	input := s.buildInput(tc, project)
	inputJSON, _ := json.Marshal(input)

	now := time.Now()
	task := &model.TestTask{
		IssueID:   testCaseID,
		ProjectID: tc.ProductID,
		AgentID:   resolvedAgentID,
		SkillName: s.resolveWorkflowName(resolvedAgent),
		Status:    model.TaskStatusRunning,
		StartedAt: &now,
		CreatedBy: createdBy,
		AIInput:   model.JSON(inputJSON),
	}

	if err := s.testTaskRepo.Create(task); err != nil {
		s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusSynced)
		return nil, fmt.Errorf("创建测试任务失败: %w", err)
	}

	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStatus,
		Status:  model.TaskStatusRunning,
		Message: "用例脚本生成任务已创建",
		Data: map[string]any{
			"test_case_id": testCaseID,
			"project_id":   tc.ProductID,
		},
	})

	go func(taskID uint64) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := s.runTask(ctx, taskID); err != nil {
			s.logger.Error("用例脚本生成失败", zap.Uint64("task_id", taskID), zap.Error(err))
			s.markTaskFailed(taskID, err)
			s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusFailed)
		} else {
			s.tcRepo.UpdateTestStatus(testCaseID, model.ZentaoTestCaseStatusGenerated)
		}
	}(task.ID)

	return task, nil
}

func (s *ZentaoTestCaseGenService) runTask(ctx context.Context, taskID uint64) error {
	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Project == nil {
		return fmt.Errorf("任务缺少项目信息")
	}

	if len(task.AIInput) == 0 {
		return fmt.Errorf("任务输入为空")
	}

	var input GenerateScriptInput
	if err := json.Unmarshal(task.AIInput, &input); err != nil {
		return fmt.Errorf("解析任务输入失败: %w", err)
	}

	var agent *model.Agent
	if task.AgentID != nil {
		agent, err = s.agentRepo.GetByID(*task.AgentID)
		if err != nil {
			return fmt.Errorf("Agent不存在: %w", err)
		}
	}

	genInput := &GenTestInput{
		ProjectID:     task.ProjectID,
		ProjectName:   task.Project.Name,
		FuncDocPath:   task.Project.FuncDocPath,
		DesignDocPath: task.Project.DesignDocPath,
		DBDocPath:     task.Project.DBDocPath,
		TestDocPath:   task.Project.TestDocPath,
		IssueTitle:    input.TestCaseTitle,
		IssueDesc:     "禅道测试用例\n\n前置条件:\n" + input.Precondition + "\n\n测试步骤:\n" + input.Steps + "\n\n预期结果:\n" + input.Expected,
		IssueSeverity: "",
	}

	output, err := s.einoRuntime.Generate(ctx, task, genInput, nil, agent, nil)
	if err != nil {
		return fmt.Errorf("Eino生成失败: %w", err)
	}

	if err := s.saveOutput(taskID, input.TestCaseID, task.ProjectID, output); err != nil {
		return fmt.Errorf("保存生成结果失败: %w", err)
	}

	now := time.Now()
	task.Status = model.TaskStatusCompleted
	task.CompletedAt = &now
	task.ErrorMessage = ""
	if err := s.testTaskRepo.Update(task); err != nil {
		return fmt.Errorf("更新任务结果失败: %w", err)
	}

	return nil
}

func (s *ZentaoTestCaseGenService) buildInput(tc *model.ZentaoTestCase, project *model.Project) GenerateScriptInput {
	return GenerateScriptInput{
		TestCaseID:    tc.ID,
		TestCaseTitle: tc.Title,
		Precondition:  tc.Precondition,
		Steps:         tc.Steps,
		Expected:      tc.Expected,
		Type:          tc.Type,
		Priority:      tc.Priority,
		ProjectID:     tc.ProductID,
		ProjectName:   project.Name,
		FuncDocPath:   project.FuncDocPath,
		DesignDocPath: project.DesignDocPath,
		DBDocPath:     project.DBDocPath,
		TestDocPath:   project.TestDocPath,
	}
}

func (s *ZentaoTestCaseGenService) resolveAgentID(agentID *uint64) (*uint64, error) {
	if agentID != nil && *agentID > 0 {
		return agentID, nil
	}

	agent, err := s.agentRepo.GetFirstActive()
	if err != nil {
		return nil, fmt.Errorf("查询Agent失败: %w", err)
	}

	return &agent.ID, nil
}

func (s *ZentaoTestCaseGenService) resolveWorkflowName(agent *model.Agent) string {
	if agent != nil && len(agent.Skills) > 0 {
		return agent.Skills[0].Name
	}
	return "gen-test"
}

func (s *ZentaoTestCaseGenService) saveOutput(taskID, testCaseID, projectID uint64, output *GenTestOutput) error {
	for _, tc := range output.TestCases {
		priority := int8(tc.Priority)
		testCase := &model.TestCase{
			TaskID:    taskID,
			IssueID:   testCaseID,
			ProjectID: projectID,
			Title:     tc.Title,
			Category:  tc.Category,
			Steps:     tc.Steps,
			Expected:  tc.Expected,
			Priority:  priority,
			Source:    "zentao_case",
		}
		if err := s.testTaskRepo.CreateTestCase(testCase); err != nil {
			s.logger.Warn("保存测试用例失败", zap.Error(err))
		}
	}

	if output.TestScript.FilePath != "" {
		script := &model.TestScript{
			TaskID:      taskID,
			IssueID:     testCaseID,
			ProjectID:   projectID,
			FilePath:    output.TestScript.FilePath,
			FileContent: output.TestScript.FileContent,
			Language:    output.TestScript.Language,
			Source:      "zentao_case",
		}
		if err := s.testTaskRepo.CreateTestScript(script); err != nil {
			s.logger.Warn("保存测试脚本失败", zap.Error(err))
		}
	}

	if output.TestDoc.Title != "" {
		doc := &model.TestDocument{
			TaskID:  taskID,
			IssueID: testCaseID,
			Title:   output.TestDoc.Title,
			Content: output.TestDoc.Content,
			DocType: "test_case_doc",
			Source:  "zentao_case",
		}
		if err := s.testTaskRepo.CreateTestDocument(doc); err != nil {
			s.logger.Warn("保存测试文档失败", zap.Error(err))
		}
	}

	return nil
}

func (s *ZentaoTestCaseGenService) publishTaskEvent(taskID uint64, event TaskEvent) {
	s.eventHub.Publish(taskID, event)
}

func (s *ZentaoTestCaseGenService) markTaskFailed(taskID uint64, err error) {
	task, getErr := s.testTaskRepo.GetByID(taskID)
	if getErr != nil {
		return
	}
	errMsg := err.Error()
	if len(errMsg) > 1000 {
		errMsg = errMsg[:1000]
	}
	now := time.Now()
	task.Status = model.TaskStatusFailed
	task.CompletedAt = &now
	task.ErrorMessage = errMsg
	_ = s.testTaskRepo.Update(task)
}
