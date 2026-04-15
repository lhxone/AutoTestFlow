package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type CIService struct {
	executionRepo *repository.ExecutionRepo
	projectRepo   *repository.ProjectRepo
	logger        *zap.Logger
	httpClient    *http.Client
}

func NewCIService(logger *zap.Logger) *CIService {
	return &CIService{
		executionRepo: repository.NewExecutionRepo(),
		projectRepo:   repository.NewProjectRepo(),
		logger:        logger,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// TriggerExecution 触发测试执行
// 创建执行记录，调用 GitLab CI API 触发 Pipeline
func (s *CIService) TriggerExecution(projectID uint64, branch string, triggerBy *uint64) (*model.TestExecution, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	if branch == "" {
		branch = project.GitBranch
	}

	// 创建执行记录
	now := time.Now()
	triggerType := "manual"
	if triggerBy == nil {
		triggerType = "schedule"
	}

	exec := &model.TestExecution{
		ProjectID:   projectID,
		TriggerType: triggerType,
		TriggerBy:   triggerBy,
		Branch:      branch,
		Status:      "pending",
		StartedAt:   &now,
	}

	if err := s.executionRepo.Create(exec); err != nil {
		return nil, err
	}

	// 尝试触发 GitLab CI Pipeline
	go s.triggerGitLabPipeline(project, exec, branch)

	return exec, nil
}

// triggerGitLabPipeline 调用 GitLab CI API 触发 Pipeline
// [MOCK] 当 GitLab CI 未配置时，模拟执行
func (s *CIService) triggerGitLabPipeline(project *model.Project, exec *model.TestExecution, branch string) {
	if project.GitRepoURL == "" {
		s.logger.Warn("项目未配置Git仓库，模拟CI执行")
		s.mockExecution(exec)
		return
	}

	// GitLab API: POST /api/v4/projects/:id/trigger/pipeline
	// 需要在 GitLab 中配置 CI Trigger Token
	// TODO: 从项目配置中获取 GitLab project ID 和 trigger token
	s.logger.Info("触发GitLab Pipeline",
		zap.Uint64("execution_id", exec.ID),
		zap.String("branch", branch))

	// 更新状态为运行中
	exec.Status = "running"
	_ = s.executionRepo.Update(exec)

	// 实际触发逻辑（需要配置 GitLab CI Token）
	// 当前为占位，等待 CI 回调更新结果
}

// mockExecution Mock CI执行（开发测试用）
// [MOCK]
func (s *CIService) mockExecution(exec *model.TestExecution) {
	time.Sleep(3 * time.Second) // 模拟执行时间

	now := time.Now()
	exec.Status = "passed"
	exec.TotalCases = 4
	exec.PassedCases = 3
	exec.FailedCases = 1
	exec.PassRate = 75.0
	exec.DurationSec = 12
	exec.CompletedAt = &now

	_ = s.executionRepo.Update(exec)
	s.logger.Info("Mock CI执行完成", zap.Uint64("execution_id", exec.ID))
}

// triggerGitLabAPI 调用 GitLab Trigger API
func (s *CIService) triggerGitLabAPI(gitlabURL, projectID, triggerToken, branch string, variables map[string]string) error {
	payload := map[string]interface{}{
		"ref":       branch,
		"token":     triggerToken,
		"variables": variables,
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v4/projects/%s/trigger/pipeline", gitlabURL, projectID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("调用GitLab API失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitLab API返回错误: %d", resp.StatusCode)
	}

	return nil
}
