package handler

import (
	"context"
	"strconv"
	"sync"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// IntegrationHandler 流水线集成接口处理器
type IntegrationHandler struct {
	issueRepo     *repository.IssueRepo
	projectRepo   *repository.ProjectRepo
	settingRepo   *repository.SettingRepo
	genTestService *service.GenTestService
	logger        *zap.Logger
}

// NewIntegrationHandler 创建集成处理器
func NewIntegrationHandler(logger *zap.Logger) *IntegrationHandler {
	return &IntegrationHandler{
		issueRepo:      repository.NewIssueRepo(),
		projectRepo:    repository.NewProjectRepo(),
		settingRepo:    repository.NewSettingRepo(),
		genTestService: service.NewGenTestService(logger),
		logger:         logger,
	}
}

// DevFlowSubmitRequest DevFlow提交通知请求
type DevFlowSubmitRequest struct {
	ZentaoIssueID     int    `json:"zentao_issue_id"`
	DevFlowSubmitTime string `json:"dev_flow_submit_time"`
	DevTaskID         string `json:"dev_task_id"`
}

// DevFlowSubmitResponse DevFlow提交通知响应
type DevFlowSubmitResponse struct {
	Code    uint64 `json:"code"`
	Message string `json:"message"`
}

// DevFlowSubmit DevFlow提交通知接口
// POST /api/integration/devflow-submit
func (h *IntegrationHandler) DevFlowSubmit(c *gin.Context) {
	var req DevFlowSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	if req.ZentaoIssueID == 0 {
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "zentao_issue_id 不能为空"})
		return
	}

	if req.DevFlowSubmitTime == "" {
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "dev_flow_submit_time 不能为空"})
		return
	}

	// 解析提交时间
	submitTime, err := time.Parse(time.RFC3339, req.DevFlowSubmitTime)
	if err != nil {
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "dev_flow_submit_time 格式错误，应为RFC3339格式"})
		return
	}

	// 查找对应的issue
	issue, err := h.issueRepo.FindByZentaoIssueID(req.ZentaoIssueID)
	if err != nil {
		h.logger.Warn("DevFlow提交通知：未找到对应的禅道问题单",
			zap.Int("zentao_issue_id", req.ZentaoIssueID),
			zap.Error(err))
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "未找到对应的禅道问题单"})
		return
	}

	// 更新issue状态和提交时间
	if err := h.issueRepo.UpdateDevFlowSubmitTime(issue.ID, submitTime, req.DevTaskID); err != nil {
		h.logger.Error("DevFlow提交通知：更新问题单失败",
			zap.Uint64("issue_id", issue.ID),
			zap.Error(err))
		c.JSON(200, DevFlowSubmitResponse{Code: 1, Message: "更新问题单失败"})
		return
	}

	h.logger.Info("DevFlow提交通知处理完成",
		zap.Uint64("issue_id", issue.ID),
		zap.Int("zentao_issue_id", req.ZentaoIssueID),
		zap.Time("submit_time", submitTime),
		zap.String("dev_task_id", req.DevTaskID))

	c.JSON(200, DevFlowSubmitResponse{Code: 0, Message: "success"})
}

// CICDDeployRequest CI/CD部署通知请求
type CICDDeployRequest struct {
	CIStartTIme      *string `json:"ci_start_time"`
	CIEndTime        *string `json:"ci_end_time"`
	CICommitShortSHA string  `json:"ci_commit_short_sha"`
}

// CICDDeployResponse CI/CD部署通知响应
type CICDDeployResponse struct {
	Code          uint64 `json:"code"`
	Message       string `json:"message"`
	TestTriggered bool   `json:"test_triggered,omitempty"`
}

// CICDDeploy CI/CD部署通知接口
// POST /api/integration/cicd-deploy
func (h *IntegrationHandler) CICDDeploy(c *gin.Context) {
	var req CICDDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, CICDDeployResponse{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}

	// 如果没有ci_end_time，表示部署开始
	if req.CIEndTime == nil || *req.CIEndTime == "" {
		h.handleDeployStart(c, req)
		return
	}

	// 有ci_end_time，表示部署完成
	h.handleDeployComplete(c, req)
}

// handleDeployStart 处理部署开始通知
func (h *IntegrationHandler) handleDeployStart(c *gin.Context, req CICDDeployRequest) {
	var startTime *time.Time
	if req.CIStartTIme != nil && *req.CIStartTIme != "" {
		t, err := time.Parse(time.RFC3339, *req.CIStartTIme)
		if err != nil {
			c.JSON(200, CICDDeployResponse{Code: 1, Message: "ci_start_time 格式错误"})
			return
		}
		startTime = &t
	}

	h.logger.Info("CI/CD部署开始通知",
		zap.Any("ci_start_time", startTime),
		zap.String("commit_sha", req.CICommitShortSHA))

	c.JSON(200, CICDDeployResponse{
		Code:          0,
		Message:       "success",
		TestTriggered: false,
	})
}

// handleDeployComplete 处理部署完成通知
func (h *IntegrationHandler) handleDeployComplete(c *gin.Context, req CICDDeployRequest) {
	// 解析部署完成时间
	endTime, err := time.Parse(time.RFC3339, *req.CIEndTime)
	if err != nil {
		c.JSON(200, CICDDeployResponse{Code: 1, Message: "ci_end_time 格式错误"})
		return
	}

	// 解析部署开始时间
	var startTime time.Time
	if req.CIStartTIme != nil && *req.CIStartTIme != "" {
		t, err := time.Parse(time.RFC3339, *req.CIStartTIme)
		if err == nil {
			startTime = t
		} else {
			startTime = endTime
		}
	} else {
		startTime = endTime
	}

	h.logger.Info("CI/CD部署完成通知",
		zap.Time("ci_start_time", startTime),
		zap.Time("ci_end_time", endTime),
		zap.String("commit_sha", req.CICommitShortSHA))

	// 查询满足条件的问题单：提交时间在部署开始之前，且状态为待升级
	issues, err := h.issueRepo.FindPendingUpgradeBeforeTime(startTime)
	if err != nil {
		h.logger.Error("查询待升级问题单失败", zap.Error(err))
		c.JSON(200, CICDDeployResponse{Code: 1, Message: "查询待升级问题单失败"})
		return
	}

	if len(issues) == 0 {
		h.logger.Info("没有待升级的问题单")
		c.JSON(200, CICDDeployResponse{
			Code:          0,
			Message:       "success",
			TestTriggered: false,
		})
		return
	}

	// 获取并行生成任务数量配置
	maxConcurrent := h.getMaxConcurrentTasks()

	h.logger.Info("找到待升级的问题单，准备触发测试任务",
		zap.Int("issue_count", len(issues)),
		zap.Int("max_concurrent", maxConcurrent))

	// 先将所有问题单状态更新为待生成
	var issueIDs []uint64
	for _, issue := range issues {
		issueIDs = append(issueIDs, issue.ID)
	}
	if err := h.issueRepo.BatchUpdateTestStatus(issueIDs, model.TestStatusPendingGenerate); err != nil {
		h.logger.Error("批量更新问题单状态失败", zap.Error(err))
		c.JSON(200, CICDDeployResponse{Code: 1, Message: "更新问题单状态失败"})
		return
	}

	// 使用信号量控制并发数
	go h.triggerTestTasks(issues, maxConcurrent)

	c.JSON(200, CICDDeployResponse{
		Code:          0,
		Message:       "success",
		TestTriggered: true,
	})
}

// triggerTestTasks 触发测试任务
func (h *IntegrationHandler) triggerTestTasks(issues []model.Issue, maxConcurrent int) {
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, issue := range issues {
		wg.Add(1)
		go func(issueID uint64, issueTitle string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			h.logger.Info("开始为问题单触发测试任务",
				zap.Uint64("issue_id", issueID))

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			task, err := h.genTestService.Execute(issueID, nil, nil, "")
			if err != nil {
				h.logger.Error("触发测试任务失败",
					zap.Uint64("issue_id", issueID),
					zap.Error(err))
				return
			}

			h.logger.Info("测试任务已创建",
				zap.Uint64("issue_id", issueID),
				zap.Uint64("task_id", task.ID))

			// 等待任务完成
			_ = h.genTestService.RunTask(ctx, task.ID)
		}(issue.ID, issue.Title)
	}

	wg.Wait()
	h.logger.Info("所有测试任务触发完成", zap.Int("total", len(issues)))
}

// getMaxConcurrentTasks 获取并行生成任务数量配置
func (h *IntegrationHandler) getMaxConcurrentTasks() int {
	value := h.settingRepo.GetValue("integration", "max_concurrent_tasks")
	if value == "" {
		return 1 // 默认值
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 1
	}
	return n
}
