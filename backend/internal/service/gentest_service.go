package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type GenTestService struct {
	testTaskRepo *repository.TestTaskRepo
	issueRepo    *repository.IssueRepo
	projectRepo  *repository.ProjectRepo
	agentRepo    *repository.AgentRepo
	skillRepo    *repository.SkillRepo
	reviewRepo   *repository.ReviewRepo
	einoRuntime  *EinoGenTestRuntime
	eventHub     *TaskEventHub
	logger       *zap.Logger
	workflow     *genTestWorkflowEngine
}

func NewGenTestService(logger *zap.Logger) *GenTestService {
	return &GenTestService{
		testTaskRepo: repository.NewTestTaskRepo(),
		issueRepo:    repository.NewIssueRepo(),
		projectRepo:  repository.NewProjectRepo(),
		agentRepo:    repository.NewAgentRepo(),
		skillRepo:    repository.NewSkillRepo(),
		reviewRepo:   repository.NewReviewRepo(),
		einoRuntime:  NewEinoGenTestRuntime(logger),
		eventHub:     DefaultTaskEventHub,
		logger:       logger,
		workflow:     newGenTestWorkflowEngine(),
	}
}

const defaultWorkflowName = "gen-test"

// GenTestInput AI生成测试的输入上下文
type GenTestInput struct {
	ProjectID        uint64 `json:"project_id"`
	IssueID          uint64 `json:"issue_id"`
	ProjectName      string `json:"project_name"`
	FuncDocPath      string `json:"func_doc_path"`
	DesignDocPath    string `json:"design_doc_path"`
	DBDocPath        string `json:"db_doc_path"`
	TestDocPath      string `json:"test_doc_path"`
	ExtraFilesPath   string `json:"extra_files_path"`
	FuncDocContent   string `json:"func_doc_content"`
	DesignDocContent string `json:"design_doc_content"`
	DBDocContent     string `json:"db_doc_content"`
	TestDocContent   string `json:"test_doc_content"`
	IssueTitle       string `json:"issue_title"`
	IssueDesc        string `json:"issue_description"`
	IssueSeverity    string `json:"issue_severity"`
}

// GenTestOutput AI生成测试的输出
type GenTestOutput struct {
	TestCases  []GenTestCase        `json:"test_cases"`
	TestScript GenTestScript        `json:"test_script"`
	TestDoc    GenTestDoc           `json:"test_doc"`
	SelfTest   *SelfTestReport      `json:"self_test,omitempty"`
	Workspace  *RuntimeWorkspace    `json:"workspace,omitempty"`
	Workflow   *GenTestWorkflowMeta `json:"workflow,omitempty"`
	Summary    string               `json:"summary"`
}

type GenTestWorkflowMeta struct {
	Engine               string   `json:"engine"`
	Name                 string   `json:"name"`
	ChromeMCPEnabled     bool     `json:"chrome_mcp_enabled"`
	ChromeMCPServers     []string `json:"chrome_mcp_servers,omitempty"`
	MCPCapabilitySummary string   `json:"mcp_capability_summary,omitempty"`
}

type GenTestCase struct {
	Title          string `json:"title"`
	Category       string `json:"category"`
	Precondition   string `json:"precondition"`
	Steps          string `json:"steps"`
	Expected       string `json:"expected"`
	SelfTestResult string `json:"self_test_result"`
	Priority       int    `json:"priority"`
}

type GenTestScript struct {
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
	Language    string `json:"language"`
}

type GenTestDoc struct {
	Title    string `json:"title"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type WorkflowRuntimeConfig struct {
	SelfTestEnabled bool
	WaitForReview   bool
	ReviewTimeout   time.Duration
}

type SelfTestReport struct {
	Passed  bool     `json:"passed"`
	Summary string   `json:"summary"`
	Checks  []string `json:"checks,omitempty"`
}

// Execute 执行gen-test任务
// 这是核心方法：接收问题单，调用AI，生成测试内容，并在后台完成自测与收尾
func (s *GenTestService) Execute(issueID uint64, agentID *uint64, createdBy *uint64, workflowName string) (*model.TestTask, error) {
	task, err := s.CreatePendingTask(issueID, agentID, createdBy, workflowName)
	if err != nil {
		return nil, err
	}

	go func(taskID uint64) {
		// 给整个异步流程设置超时上限（30 分钟），避免无限挂起
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := s.RunTask(ctx, taskID); err != nil {
			s.logger.Error("异步执行gen-test任务失败", zap.Uint64("task_id", taskID), zap.Error(err))
			s.markTaskFailedIfStillRunning(taskID, err)
		}
	}(task.ID)

	return task, nil
}

// ExecuteSync 同步执行gen-test任务
func (s *GenTestService) ExecuteSync(ctx context.Context, issueID uint64, agentID *uint64, createdBy *uint64, workflowName string) (*model.TestTask, error) {
	task, err := s.CreatePendingTask(issueID, agentID, createdBy, workflowName)
	if err != nil {
		return nil, err
	}

	if err := s.RunTask(ctx, task.ID); err != nil {
		s.markTaskFailedIfStillRunning(task.ID, err)
		return nil, err
	}

	return s.testTaskRepo.GetByID(task.ID)
}

// CreatePendingTask 创建待执行的测试任务，供 Temporal workflow 启动前持久化任务使用
func (s *GenTestService) CreatePendingTask(issueID uint64, agentID *uint64, createdBy *uint64, workflowName string) (*model.TestTask, error) {
	issue, err := s.issueRepo.GetByID(issueID)
	if err != nil {
		return nil, fmt.Errorf("问题单不存在: %w", err)
	}

	project, err := s.projectRepo.GetByID(issue.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	// 更新问题单测试状态为"生成中"
	_ = s.issueRepo.ForceUpdateTestStatus(issueID, model.TestStatusGenerating)

	resolvedAgentID, err := s.resolveRequestedAgentID(agentID)
	if err != nil {
		return nil, err
	}

	var resolvedAgent *model.Agent
	if resolvedAgentID != nil {
		resolvedAgent, err = s.agentRepo.GetByID(*resolvedAgentID)
		if err != nil {
			return nil, fmt.Errorf("Agent不存在: %w", err)
		}
	}

	// 创建测试任务
	now := time.Now()
	task := &model.TestTask{
		IssueID:   issueID,
		ProjectID: project.ID,
		AgentID:   resolvedAgentID,
		SkillName: s.resolveTaskWorkflowName(workflowName, resolvedAgent),
		Status:    model.TaskStatusRunning,
		StartedAt: &now,
		CreatedBy: createdBy,
	}

	// 构建AI输入上下文
	input := s.buildInput(project, issue)
	inputJSON, _ := json.Marshal(input)
	task.AIInput = model.JSON(inputJSON)

	if err := s.testTaskRepo.Create(task); err != nil {
		return nil, err
	}

	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStatus,
		Status:  model.TaskStatusRunning,
		Message: "测试任务已创建，等待 Eino Runtime 执行",
		Data: map[string]any{
			"issue_id":      task.IssueID,
			"project_id":    task.ProjectID,
			"workflow_name": task.SkillName,
		},
	})

	return task, nil
}

// RunTask 运行已创建的测试任务，适用于 Temporal activity 或后台重试
func (s *GenTestService) RunTask(ctx context.Context, taskID uint64) error {
	runner, err := s.workflow.get(s)
	if err != nil {
		return err
	}

	_, err = runner.Invoke(ctx, taskID)
	if err != nil {
		s.markTaskFailedIfStillRunning(taskID, err)
		return err
	}

	return nil
}

func (s *GenTestService) loadInputForTask(task *model.TestTask) (*GenTestInput, error) {
	if len(task.AIInput) > 0 {
		var input GenTestInput
		if err := json.Unmarshal(task.AIInput, &input); err == nil {
			return &input, nil
		}
	}

	issue, err := s.issueRepo.GetByID(task.IssueID)
	if err != nil {
		return nil, fmt.Errorf("问题单不存在: %w", err)
	}
	project, err := s.projectRepo.GetByID(task.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	input := s.buildInput(project, issue)
	inputJSON, _ := json.Marshal(input)
	task.AIInput = model.JSON(inputJSON)
	_ = s.testTaskRepo.Update(task)
	return input, nil
}

// runGenerateTask 调用 Eino Runtime 生成并持久化测试内容。
func (s *GenTestService) runGenerateTask(
	ctx context.Context,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) error {
	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "runtime_start",
		Status:  model.TaskStatusRunning,
		Message: "开始执行 Eino Runtime",
	})
	output, err := s.einoRuntime.Generate(ctx, task, input, workflow, agent, promptCtx)
	if err != nil {
		return err
	}

	normalizeGeneratedTestCases(output)
	output.Workflow = buildWorkflowMeta(promptCtx)
	if err := s.persistGeneratedArtifacts(task, output); err != nil {
		return err
	}

	s.logger.Info("Eino Runtime 生成测试完成，等待自测收尾",
		zap.Uint64("task_id", task.ID),
		zap.String("workflow_name", task.SkillName))

	return nil
}

func (s *GenTestService) resolveRequestedAgentID(agentID *uint64) (*uint64, error) {
	if agentID != nil {
		agent, err := s.agentRepo.GetByID(*agentID)
		if err != nil {
			return nil, fmt.Errorf("Agent不存在: %w", err)
		}
		if agent.Status == 0 {
			return nil, fmt.Errorf("Agent已禁用: %s", agent.Name)
		}
		resolved := agent.ID
		return &resolved, nil
	}

	defaultAgent, err := s.agentRepo.GetDefaultActive()
	if err == nil {
		resolved := defaultAgent.ID
		return &resolved, nil
	}

	agent, err := s.agentRepo.GetFirstActive()
	if err != nil {
		return nil, nil
	}
	resolved := agent.ID
	return &resolved, nil
}

func (s *GenTestService) resolveAgentForTask(task *model.TestTask) (*model.Agent, error) {
	if task.AgentID != nil {
		agent, err := s.agentRepo.GetByID(*task.AgentID)
		if err != nil {
			return nil, fmt.Errorf("测试任务绑定的Agent不存在: %w", err)
		}
		if agent.Status == 0 {
			return nil, fmt.Errorf("测试任务绑定的Agent已禁用: %s", agent.Name)
		}
		return agent, nil
	}

	defaultAgent, err := s.agentRepo.GetDefaultActive()
	if err == nil {
		task.AgentID = &defaultAgent.ID
		_ = s.testTaskRepo.Update(task)
		return s.agentRepo.GetByID(defaultAgent.ID)
	}

	agent, err := s.agentRepo.GetFirstActive()
	if err != nil {
		return nil, nil
	}
	task.AgentID = &agent.ID
	_ = s.testTaskRepo.Update(task)
	return s.agentRepo.GetByID(agent.ID)
}

func (s *GenTestService) SelfTestTask(ctx context.Context, taskID uint64) (*SelfTestReport, error) {
	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("测试任务不存在: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cases, err := s.testTaskRepo.GetTestCasesByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	scripts, err := s.testTaskRepo.GetTestScriptsByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	docs, err := s.testTaskRepo.GetTestDocsByTaskID(taskID)
	if err != nil {
		return nil, err
	}

	report := extractSelfTestReport(task.AIOutput)
	if report == nil {
		report = &SelfTestReport{
			Passed: true,
			Checks: make([]string, 0, 8),
		}
	} else {
		report = cloneSelfTestReport(report)
		if report.Checks == nil {
			report.Checks = make([]string, 0, 8)
		}
	}

	if len(cases) == 0 {
		report.Passed = false
		report.Checks = append(report.Checks, "未生成测试用例")
	}
	if len(scripts) == 0 {
		report.Passed = false
		report.Checks = append(report.Checks, "未生成测试脚本")
	}
	if len(docs) == 0 {
		report.Passed = false
		report.Checks = append(report.Checks, "未生成测试文档")
	}

	for _, tc := range cases {
		tc.SelfTestResult = "pass"
		if strings.TrimSpace(tc.Title) == "" || strings.TrimSpace(tc.Steps) == "" || strings.TrimSpace(tc.Expected) == "" {
			tc.SelfTestResult = "fail"
			report.Passed = false
			report.Checks = append(report.Checks, fmt.Sprintf("测试用例 #%d 缺少标题/步骤/预期结果", tc.ID))
		}
		_ = s.testTaskRepo.UpdateTestCase(&tc)
	}

	for _, script := range NormalizeTestScripts(scripts) {
		if err := validateGeneratedScript(script); err != nil {
			report.Passed = false
			report.Checks = append(report.Checks, fmt.Sprintf("%s 自测失败: %s", script.FilePath, err.Error()))
		}
	}

	for _, doc := range docs {
		if strings.TrimSpace(doc.Content) == "" {
			report.Passed = false
			report.Checks = append(report.Checks, fmt.Sprintf("测试文档 #%d 内容为空", doc.ID))
		}
	}

	if report.Passed {
		report.Summary = fmt.Sprintf("自测通过：%d 个测试用例、%d 个脚本、%d 份文档结构检查完成", len(cases), len(scripts), len(docs))
	} else {
		if len(report.Checks) > 0 {
			report.Summary = strings.Join(report.Checks, "；")
		} else if strings.TrimSpace(report.Summary) == "" {
			report.Summary = "自测失败"
		}
		if task.ErrorMessage == "" {
			task.ErrorMessage = report.Summary
			_ = s.testTaskRepo.Update(task)
		}
	}

	return report, nil
}

func (s *GenTestService) FinalizeTask(taskID uint64, report *SelfTestReport) error {
	// 清理 Chrome MCP 占用的 profile 目录
	status, errMsg := cleanupChromeProfile()
	switch status {
	case "cleaned":
		s.publishTaskEvent(taskID, TaskEvent{
			Type:    taskEventTypeLog,
			Stage:   "chrome_profile_cleanup",
			Status:  model.TaskStatusRunning,
			Message: "已清理 Chrome profile 目录",
			Data:    map[string]any{"path": chromeProfilePath},
		})
	case "error":
		s.publishTaskEvent(taskID, TaskEvent{
			Type:    taskEventTypeLog,
			Stage:   "chrome_profile_cleanup",
			Status:  model.TaskStatusRunning,
			Message: fmt.Sprintf("清理 Chrome profile 目录失败: %s", errMsg),
			Data:    map[string]any{"path": chromeProfilePath, "error": errMsg},
		})
	case "not_exist":
		s.publishTaskEvent(taskID, TaskEvent{
			Type:    taskEventTypeLog,
			Stage:   "chrome_profile_cleanup",
			Status:  model.TaskStatusRunning,
			Message: "Chrome profile 目录不存在，跳过清理",
			Data:    map[string]any{"path": chromeProfilePath},
		})
	}

	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("测试任务不存在: %w", err)
	}

	if err := s.updateTaskSelfTestReport(task, report); err != nil {
		return err
	}

	if report != nil && !report.Passed {
		now := time.Now()
		task.Status = model.TaskStatusWarning
		task.CompletedAt = &now
		task.ErrorMessage = report.Summary
		_ = s.testTaskRepo.Update(task)
		_ = s.issueRepo.ForceUpdateTestStatus(task.IssueID, model.TestStatusReviewPending)
		s.publishTaskEvent(task.ID, TaskEvent{
			Type:    taskEventTypeStatus,
			Stage:   "self_test_failed",
			Status:  model.TaskStatusWarning,
			Message: report.Summary,
		})
		if _, err := s.ensureReviewTask(task, "测试资产已生成，自测失败，可继续编辑、审批和提交"); err != nil {
			return err
		}
		s.publishTaskEvent(task.ID, TaskEvent{
			Type:    taskEventTypeStatus,
			Stage:   "review_pending",
			Status:  model.TaskStatusWarning,
			Message: "测试资产已生成，自测未通过，已进入 Review 阶段，可继续人工处理",
		})
		return nil
	}

	now := time.Now()
	task.Status = model.TaskStatusCompleted
	task.CompletedAt = &now
	task.ErrorMessage = ""
	if err := s.testTaskRepo.Update(task); err != nil {
		return err
	}

	_ = s.issueRepo.ForceUpdateTestStatus(task.IssueID, model.TestStatusReviewPending)
	review, err := s.ensureReviewTask(task, "测试任务已生成完成，进入 Review 阶段")
	if err != nil {
		return err
	}
	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStatus,
		Stage:   "review_pending",
		Status:  model.TaskStatusCompleted,
		Message: "测试任务已生成完成，进入 Review 阶段",
	})

	s.logger.Info("测试生成任务已完成并进入Review",
		zap.Uint64("task_id", task.ID),
		zap.Uint64("review_id", review.ID),
		zap.String("workflow_name", task.SkillName))

	return nil
}

func (s *GenTestService) ResolveWorkflowRuntimeConfig(workflowName string) WorkflowRuntimeConfig {
	cfg := WorkflowRuntimeConfig{
		SelfTestEnabled: true,
	}

	workflow, err := s.loadWorkflow(workflowName)
	if err != nil || workflow == nil || len(workflow.ConfigJSON) == 0 {
		return cfg
	}

	var raw struct {
		SelfTestEnabled *bool  `json:"self_test_enabled"`
		WaitForReview   *bool  `json:"wait_for_review"`
		ReviewTimeout   string `json:"review_timeout"`
	}
	if err := json.Unmarshal(workflow.ConfigJSON, &raw); err != nil {
		return cfg
	}

	if raw.SelfTestEnabled != nil {
		cfg.SelfTestEnabled = *raw.SelfTestEnabled
	}
	if raw.WaitForReview != nil {
		cfg.WaitForReview = *raw.WaitForReview
	}
	if strings.TrimSpace(raw.ReviewTimeout) != "" {
		if d, err := time.ParseDuration(raw.ReviewTimeout); err == nil {
			cfg.ReviewTimeout = d
		}
	}

	return cfg
}

func (s *GenTestService) resolveWorkflowName(workflowName string) string {
	if strings.TrimSpace(workflowName) == "" {
		return defaultWorkflowName
	}
	return strings.TrimSpace(workflowName)
}

func (s *GenTestService) resolveTaskWorkflowName(workflowName string, agent *model.Agent) string {
	resolved := strings.TrimSpace(workflowName)
	if resolved != "" {
		return resolved
	}
	if agent == nil {
		return defaultWorkflowName
	}

	activeWorkflows := make([]string, 0, len(agent.Skills))
	for _, skill := range agent.Skills {
		name := strings.TrimSpace(skill.Name)
		if skill.Status != 0 && name != "" {
			activeWorkflows = append(activeWorkflows, name)
		}
	}

	if len(activeWorkflows) == 1 {
		return activeWorkflows[0]
	}
	if len(activeWorkflows) > 1 {
		s.logger.Warn("Agent绑定了多个启用workflow，未显式指定workflow时回退默认workflow",
			zap.Uint64("agent_id", agent.ID),
			zap.Strings("workflow_names", activeWorkflows))
	}

	return defaultWorkflowName
}

func (s *GenTestService) loadWorkflow(workflowName string) (*model.Skill, error) {
	name := s.resolveWorkflowName(workflowName)
	workflow, err := s.skillRepo.GetByName(name)
	if err != nil {
		if name == defaultWorkflowName {
			return nil, nil
		}
		return nil, fmt.Errorf("workflow不存在: %w", err)
	}
	if workflow.Status == 0 {
		return nil, fmt.Errorf("workflow已禁用: %s", name)
	}
	return workflow, nil
}

func buildDefaultDocPath(task *model.TestTask) string {
	return fmt.Sprintf("docs/issue-%d-test-case.md", task.IssueID)
}

func extractSelfTestReport(raw model.JSON) *SelfTestReport {
	if len(raw) == 0 {
		return nil
	}
	var output GenTestOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil
	}
	return output.SelfTest
}

func cloneSelfTestReport(report *SelfTestReport) *SelfTestReport {
	if report == nil {
		return nil
	}
	cloned := &SelfTestReport{
		Passed:  report.Passed,
		Summary: report.Summary,
	}
	if len(report.Checks) > 0 {
		cloned.Checks = append([]string(nil), report.Checks...)
	}
	return cloned
}

func validateGeneratedScript(script model.TestScript) error {
	content := strings.TrimSpace(script.FileContent)
	if strings.TrimSpace(script.FilePath) == "" {
		return fmt.Errorf("脚本路径为空")
	}
	if content == "" {
		return fmt.Errorf("脚本内容为空")
	}

	switch normalizeScriptLanguage(script.Language, script.FilePath) {
	case "typescript", "javascript":
		if !strings.Contains(content, "test(") && !strings.Contains(content, "test.describe(") {
			return fmt.Errorf("缺少可执行的测试声明")
		}
		if !hasJSAssertion(content, script.FilePath) {
			return fmt.Errorf("缺少断言")
		}
	case "python":
		if !strings.Contains(content, "def test_") && !strings.Contains(content, "class Test") {
			return fmt.Errorf("缺少 pytest 测试入口")
		}
	default:
		return fmt.Errorf("暂不支持的脚本语言: %s", script.Language)
	}

	return nil
}

func hasJSAssertion(content, filePath string) bool {
	lowerContent := strings.ToLower(content)
	assertionTokens := []string{
		"expect(",
		"assert(",
		"assert.",
		"should(",
		".should(",
		"verify(",
		"check(",
		"tobe(",
		"toequal(",
		"tohavetext(",
		"tohavecount(",
		"tohavevalue(",
	}
	for _, token := range assertionTokens {
		if strings.Contains(lowerContent, token) {
			return true
		}
	}

	// 场景脚本通常把断言封装在共享步骤中，避免误判为“缺少断言”。
	lowerPath := strings.ToLower(strings.TrimSpace(filePath))
	if strings.HasSuffix(lowerPath, ".scenario.ts") {
		hasScenarioEntry := strings.Contains(lowerContent, "scenario(") || strings.Contains(lowerContent, "test(")
		hasSharedFlow := strings.Contains(lowerContent, "loginflow(") || strings.Contains(lowerContent, "logoutflow(") || strings.Contains(lowerContent, "step(")
		if hasScenarioEntry && hasSharedFlow {
			return true
		}
	}

	return false
}

// buildInput 构建AI输入上下文
func (s *GenTestService) buildInput(project *model.Project, issue *model.Issue) *GenTestInput {
	input := &GenTestInput{
		ProjectID:      project.ID,
		IssueID:        issue.ID,
		ProjectName:    project.Name,
		FuncDocPath:    project.FuncDocPath,
		DesignDocPath:  project.DesignDocPath,
		DBDocPath:      project.DBDocPath,
		TestDocPath:    project.TestDocPath,
		ExtraFilesPath: project.ExtraFilesPath,
		IssueTitle:     issue.Title,
		IssueDesc:      issue.Description,
		IssueSeverity:  issue.Severity,
	}

	return input
}

// markTaskFailedIfStillRunning 兜底：如果任务仍处于 running 状态，标记为失败并发布错误事件。
// 防止 RunTask 在 runGenerateTask 之前就返回错误时，任务永远卡在 running。
func (s *GenTestService) markTaskFailedIfStillRunning(taskID uint64, reason error) {
	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return
	}
	if task.Status != model.TaskStatusRunning {
		return // 已经被 runGenerateTask 等流程处理过了
	}

	if s.taskHasGeneratedArtifacts(task) {
		now := time.Now()
		task.Status = model.TaskStatusWarning
		task.ErrorMessage = reason.Error()
		task.CompletedAt = &now
		_ = s.testTaskRepo.Update(task)
		_ = s.issueRepo.ForceUpdateTestStatus(task.IssueID, model.TestStatusReviewPending)
		if _, ensureErr := s.ensureReviewTask(task, "生成阶段存在异常，但已保留当前产物，可继续编辑、审批和提交"); ensureErr == nil {
			s.publishTaskEvent(taskID, TaskEvent{
				Type:    taskEventTypeError,
				Stage:   "runtime_failed",
				Status:  model.TaskStatusWarning,
				Message: reason.Error(),
			})
			s.publishTaskEvent(taskID, TaskEvent{
				Type:    taskEventTypeStatus,
				Stage:   "review_pending",
				Status:  model.TaskStatusWarning,
				Message: "生成过程中出现异常，但当前产物已保留，已进入 Review 阶段",
			})
			return
		}
	}

	now := time.Now()
	task.Status = model.TaskStatusFailed
	task.ErrorMessage = reason.Error()
	task.CompletedAt = &now
	_ = s.testTaskRepo.Update(task)
	_ = s.issueRepo.ForceUpdateTestStatus(task.IssueID, model.TestStatusError)

	s.publishTaskEvent(taskID, TaskEvent{
		Type:    taskEventTypeError,
		Stage:   "runtime_failed",
		Status:  model.TaskStatusFailed,
		Message: reason.Error(),
	})
}

func (s *GenTestService) taskHasGeneratedArtifacts(task *model.TestTask) bool {
	if task == nil {
		return false
	}
	if len(task.AIOutput) > 0 {
		return true
	}
	cases, _ := s.testTaskRepo.GetTestCasesByTaskID(task.ID)
	if len(cases) > 0 {
		return true
	}
	scripts, _ := s.testTaskRepo.GetTestScriptsByTaskID(task.ID)
	if len(scripts) > 0 {
		return true
	}
	docs, _ := s.testTaskRepo.GetTestDocsByTaskID(task.ID)
	return len(docs) > 0
}

func (s *GenTestService) ensureReviewTask(task *model.TestTask, note string) (*model.ReviewTask, error) {
	if task == nil {
		return nil, fmt.Errorf("测试任务不能为空")
	}

	review, err := s.reviewRepo.GetLatestByTestTaskID(task.ID)
	if err == nil {
		return review, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	input, inputErr := s.loadInputForTask(task)
	title := fmt.Sprintf("Review: Task #%d", task.ID)
	if inputErr == nil && input != nil && strings.TrimSpace(input.IssueTitle) != "" {
		title = fmt.Sprintf("Review: %s", input.IssueTitle)
	}

	review = &model.ReviewTask{
		TestTaskID:  task.ID,
		IssueID:     task.IssueID,
		ProjectID:   task.ProjectID,
		Title:       title,
		Status:      model.ReviewStatusPending,
		SubmittedBy: task.CreatedBy,
		ReviewNote:  note,
	}
	if err := s.reviewRepo.Create(review); err != nil {
		return nil, err
	}

	return review, nil
}

func (s *GenTestService) persistGeneratedArtifacts(task *model.TestTask, output *GenTestOutput) error {
	if err := s.testTaskRepo.DeleteArtifactsByTaskID(task.ID); err != nil {
		return err
	}

	outputJSON, _ := json.Marshal(output)
	task.AIOutput = model.JSON(outputJSON)
	task.ErrorMessage = ""
	if err := s.testTaskRepo.Update(task); err != nil {
		return err
	}

	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStatus,
		Stage:   "runtime_completed",
		Status:  model.TaskStatusRunning,
		Message: "Eino Runtime 已完成测试资产生成",
	})

	for _, tc := range output.TestCases {
		testCase := &model.TestCase{
			TaskID:         task.ID,
			IssueID:        task.IssueID,
			ProjectID:      task.ProjectID,
			Title:          tc.Title,
			Category:       tc.Category,
			Precondition:   tc.Precondition,
			Steps:          tc.Steps,
			Expected:       tc.Expected,
			SelfTestResult: tc.SelfTestResult,
			Priority:       int8(tc.Priority),
			Source:         "ai",
		}
		if err := s.testTaskRepo.CreateTestCase(testCase); err != nil {
			return err
		}
		if err := s.testTaskRepo.CreateTestCaseVersion(&model.TestCaseVersion{
			TestCaseID:   testCase.ID,
			Version:      1,
			Title:        testCase.Title,
			Precondition: testCase.Precondition,
			Steps:        testCase.Steps,
			Expected:     testCase.Expected,
			Source:       "ai",
			ChangeNote:   "AI初始生成",
		}); err != nil {
			return err
		}
	}

	if output.TestScript.FileContent != "" {
		script := &model.TestScript{
			TaskID:      task.ID,
			IssueID:     task.IssueID,
			ProjectID:   task.ProjectID,
			FilePath:    output.TestScript.FilePath,
			FileContent: output.TestScript.FileContent,
			Language:    output.TestScript.Language,
			Source:      "ai",
		}
		if err := s.testTaskRepo.CreateTestScript(script); err != nil {
			return err
		}
		if err := s.testTaskRepo.CreateTestScriptVersion(&model.TestScriptVersion{
			TestScriptID: script.ID,
			Version:      1,
			FileContent:  script.FileContent,
			Source:       "ai",
			ChangeNote:   "AI初始生成",
		}); err != nil {
			return err
		}
	}

	if output.TestDoc.Content != "" {
		docPath := strings.TrimSpace(output.TestDoc.FilePath)
		if docPath == "" {
			docPath = buildDefaultDocPath(task)
		}
		doc := &model.TestDocument{
			TaskID:    task.ID,
			IssueID:   task.IssueID,
			ProjectID: task.ProjectID,
			Title:     output.TestDoc.Title,
			FilePath:  docPath,
			Content:   output.TestDoc.Content,
			DocType:   "test_case_doc",
			Source:    "ai",
		}
		if err := s.testTaskRepo.CreateTestDocument(doc); err != nil {
			return err
		}
	}

	return nil
}

func (s *GenTestService) updateTaskSelfTestReport(task *model.TestTask, report *SelfTestReport) error {
	if task == nil || report == nil {
		return nil
	}

	var output GenTestOutput
	if len(task.AIOutput) > 0 {
		_ = json.Unmarshal(task.AIOutput, &output)
	}
	output.SelfTest = cloneSelfTestReport(report)

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return err
	}
	task.AIOutput = model.JSON(outputJSON)
	return s.testTaskRepo.Update(task)
}

func (s *GenTestService) publishTaskEvent(taskID uint64, event TaskEvent) {
	if s.eventHub == nil {
		return
	}
	s.eventHub.Publish(taskID, event)
}

func buildWorkflowMeta(promptCtx *CLIPromptContext) *GenTestWorkflowMeta {
	meta := &GenTestWorkflowMeta{
		Engine: "eino",
		Name:   "gen-test-eino-workflow",
	}
	if promptCtx == nil {
		return meta
	}
	if len(promptCtx.ChromeMCPServers) > 0 {
		meta.ChromeMCPEnabled = true
		meta.ChromeMCPServers = append([]string(nil), promptCtx.ChromeMCPServers...)
	}
	meta.MCPCapabilitySummary = strings.TrimSpace(promptCtx.MCPCapabilitySummary)
	return meta
}
