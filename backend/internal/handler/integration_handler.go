package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// IntegrationHandler 流水线集成接口处理器
type IntegrationHandler struct {
	issueRepo      *repository.IssueRepo
	projectRepo    *repository.ProjectRepo
	settingRepo    *repository.SettingRepo
	apiLogRepo     *repository.APIExchangeLogRepo
	genTestService *service.GenTestService
	projectService *service.ProjectService
	logger         *zap.Logger
}

// NewIntegrationHandler 创建集成处理器
func NewIntegrationHandler(logger *zap.Logger) *IntegrationHandler {
	return &IntegrationHandler{
		issueRepo:      repository.NewIssueRepo(),
		projectRepo:    repository.NewProjectRepo(),
		settingRepo:    repository.NewSettingRepo(),
		apiLogRepo:     repository.NewAPIExchangeLogRepo(),
		genTestService: service.NewGenTestService(logger),
		projectService: service.NewProjectService(),
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
	start := time.Now()
	rawBody, bodyErr := readRequestBody(c)
	var req DevFlowSubmitRequest
	if bodyErr != nil {
		resp := DevFlowSubmitResponse{Code: 1, Message: "读取请求体失败: " + bodyErr.Error()}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, bodyErr.Error(), nil)
		return
	}
	if err := bindJSONBody(c, rawBody, &req); err != nil {
		resp := DevFlowSubmitResponse{Code: 1, Message: "参数错误: " + err.Error()}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, err.Error(), nil)
		return
	}

	if req.ZentaoIssueID == 0 {
		resp := DevFlowSubmitResponse{Code: 1, Message: "zentao_issue_id 不能为空"}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
		return
	}

	if req.DevFlowSubmitTime == "" {
		resp := DevFlowSubmitResponse{Code: 1, Message: "dev_flow_submit_time 不能为空"}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
		return
	}

	// 解析提交时间
	submitTime, err := time.Parse(time.RFC3339, req.DevFlowSubmitTime)
	if err != nil {
		resp := DevFlowSubmitResponse{Code: 1, Message: "dev_flow_submit_time 格式错误，应为RFC3339格式"}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
		return
	}

	// 查找对应的issue
	issue, err := h.issueRepo.FindByZentaoIssueID(req.ZentaoIssueID)
	if err != nil {
		h.logger.Warn("DevFlow提交通知：未找到对应的禅道问题单",
			zap.Int("zentao_issue_id", req.ZentaoIssueID),
			zap.Error(err))
		resp := DevFlowSubmitResponse{Code: 1, Message: "未找到对应的禅道问题单"}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
		return
	}

	// 更新issue状态和提交时间
	if err := h.issueRepo.UpdateDevFlowSubmitTime(issue.ID, submitTime, req.DevTaskID); err != nil {
		h.logger.Error("DevFlow提交通知：更新问题单失败",
			zap.Uint64("issue_id", issue.ID),
			zap.Error(err))
		resp := DevFlowSubmitResponse{Code: 1, Message: "更新问题单失败"}
		h.respondDevFlowSubmit(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, err.Error(), &issue.ID)
		return
	}

	h.logger.Info("DevFlow提交通知处理完成",
		zap.Uint64("issue_id", issue.ID),
		zap.Int("zentao_issue_id", req.ZentaoIssueID),
		zap.Time("submit_time", submitTime),
		zap.String("dev_task_id", req.DevTaskID))

	h.respondDevFlowSubmit(c, start, rawBody, req, DevFlowSubmitResponse{Code: 0, Message: "success"}, model.APIExchangeStatusSuccess, "", &issue.ID)
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

type ProjectMetricsResponse struct {
	Code    uint64 `json:"code"`
	Message string `json:"message"`
	Data    gin.H  `json:"data,omitempty"`
}

// CICDDeploy CI/CD部署通知接口
// POST /api/integration/cicd-deploy
func (h *IntegrationHandler) CICDDeploy(c *gin.Context) {
	start := time.Now()
	rawBody, bodyErr := readRequestBody(c)
	var req CICDDeployRequest
	if bodyErr != nil {
		resp := CICDDeployResponse{Code: 1, Message: "读取请求体失败: " + bodyErr.Error()}
		h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, bodyErr.Error(), nil)
		return
	}
	if err := bindJSONBody(c, rawBody, &req); err != nil {
		resp := CICDDeployResponse{Code: 1, Message: "参数错误: " + err.Error()}
		h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, err.Error(), nil)
		return
	}

	// 如果没有ci_end_time，表示部署开始
	if req.CIEndTime == nil || *req.CIEndTime == "" {
		h.handleDeployStart(c, start, rawBody, req)
		return
	}

	// 有ci_end_time，表示部署完成
	h.handleDeployComplete(c, start, rawBody, req)
}

// GetProjectMetrics 查询项目维度指标
// GET /api/integration/project-metrics
func (h *IntegrationHandler) GetProjectMetrics(c *gin.Context) {
	start := time.Now()
	rawBody, bodyErr := readRequestBody(c)
	var query dto.ProjectMetricsQuery
	if bodyErr != nil {
		resp := ProjectMetricsResponse{Code: 1, Message: "读取请求体失败: " + bodyErr.Error()}
		h.respondProjectMetrics(c, start, rawBody, query, resp, model.APIExchangeStatusFailed, bodyErr.Error())
		return
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		resp := ProjectMetricsResponse{Code: 1, Message: "参数错误: " + err.Error()}
		h.respondProjectMetrics(c, start, rawBody, query, resp, model.APIExchangeStatusFailed, err.Error())
		return
	}
	if query.StartDate == "" {
		query.StartDate = strings.TrimSpace(c.Query("start_date"))
	}
	if query.EndDate == "" {
		query.EndDate = strings.TrimSpace(c.Query("end_date"))
	}

	items, err := h.projectService.GetProjectMetrics(&query)
	if err != nil {
		resp := ProjectMetricsResponse{Code: 1, Message: err.Error()}
		status := model.APIExchangeStatusFailed
		h.respondProjectMetrics(c, start, rawBody, query, resp, status, err.Error())
		return
	}

	resp := ProjectMetricsResponse{
		Code:    0,
		Message: "success",
		Data:    gin.H{"list": items},
	}
	h.respondProjectMetrics(c, start, rawBody, query, resp, model.APIExchangeStatusSuccess, "")
}

// handleDeployStart 处理部署开始通知
func (h *IntegrationHandler) handleDeployStart(c *gin.Context, start time.Time, rawBody string, req CICDDeployRequest) {
	var startTime *time.Time
	if req.CIStartTIme != nil && *req.CIStartTIme != "" {
		t, err := time.Parse(time.RFC3339, *req.CIStartTIme)
		if err != nil {
			resp := CICDDeployResponse{Code: 1, Message: "ci_start_time 格式错误"}
			h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
			return
		}
		startTime = &t
	}

	h.logger.Info("CI/CD部署开始通知",
		zap.Any("ci_start_time", startTime),
		zap.String("commit_sha", req.CICommitShortSHA))

	h.respondCICDDeploy(c, start, rawBody, req, CICDDeployResponse{
		Code:          0,
		Message:       "success",
		TestTriggered: false,
	}, model.APIExchangeStatusSuccess, "", nil)
}

// handleDeployComplete 处理部署完成通知
func (h *IntegrationHandler) handleDeployComplete(c *gin.Context, start time.Time, rawBody string, req CICDDeployRequest) {
	// 解析部署完成时间
	endTime, err := time.Parse(time.RFC3339, *req.CIEndTime)
	if err != nil {
		resp := CICDDeployResponse{Code: 1, Message: "ci_end_time 格式错误"}
		h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, resp.Message, nil)
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
		resp := CICDDeployResponse{Code: 1, Message: "查询待升级问题单失败"}
		h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, err.Error(), nil)
		return
	}

	if len(issues) == 0 {
		h.logger.Info("没有待升级的问题单")
		h.respondCICDDeploy(c, start, rawBody, req, CICDDeployResponse{
			Code:          0,
			Message:       "success",
			TestTriggered: false,
		}, model.APIExchangeStatusSuccess, "", nil)
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
		resp := CICDDeployResponse{Code: 1, Message: "更新问题单状态失败"}
		h.respondCICDDeploy(c, start, rawBody, req, resp, model.APIExchangeStatusFailed, err.Error(), issueIDs)
		return
	}

	// 使用信号量控制并发数
	go h.triggerTestTasks(issues, maxConcurrent)

	h.respondCICDDeploy(c, start, rawBody, req, CICDDeployResponse{
		Code:          0,
		Message:       "success",
		TestTriggered: true,
	}, model.APIExchangeStatusSuccess, "", issueIDs)
}

func (h *IntegrationHandler) respondDevFlowSubmit(c *gin.Context, start time.Time, rawBody string, req DevFlowSubmitRequest, resp DevFlowSubmitResponse, resultStatus, errMsg string, issueID *uint64) {
	h.recordInboundAPI(c, start, "devflow_submit", rawBody, req, resp, resultStatus, errMsg, issueID, nil, req.DevTaskID)
	c.JSON(200, resp)
}

func (h *IntegrationHandler) respondCICDDeploy(c *gin.Context, start time.Time, rawBody string, req CICDDeployRequest, resp CICDDeployResponse, resultStatus, errMsg string, issueIDs []uint64) {
	var issueID *uint64
	if len(issueIDs) == 1 {
		issueID = &issueIDs[0]
	}
	h.recordInboundAPI(c, start, "cicd_deploy", rawBody, req, resp, resultStatus, errMsg, issueID, nil, "")
	c.JSON(200, resp)
}

func (h *IntegrationHandler) respondProjectMetrics(c *gin.Context, start time.Time, rawBody string, req dto.ProjectMetricsQuery, resp ProjectMetricsResponse, resultStatus, errMsg string) {
	h.recordInboundAPI(c, start, "project_metrics", rawBody, req, resp, resultStatus, errMsg, nil, nil, "")
	c.JSON(200, resp)
}

func (h *IntegrationHandler) recordInboundAPI(c *gin.Context, start time.Time, apiName string, rawBody string, req any, resp any, resultStatus, errMsg string, issueID, taskID *uint64, devTaskID string) {
	if h.apiLogRepo == nil {
		return
	}
	if rawBody == "" {
		reqBody, _ := json.Marshal(req)
		rawBody = string(reqBody)
	}
	respBody, _ := json.Marshal(resp)
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.RequestURI()
	}
	if resultStatus == "" {
		resultStatus = model.APIExchangeStatusSuccess
	}
	log := &model.APIExchangeLog{
		APIName:          apiName,
		Direction:        model.APIExchangeDirectionInbound,
		Method:           c.Request.Method,
		URL:              path,
		RemoteAddr:       c.ClientIP(),
		RequestHeaders:   repository.JSONValue(redactHeaders(c.Request.Header)),
		RequestBody:      rawBody,
		ResponseStatus:   200,
		ResponseBody:     string(respBody),
		ResultStatus:     resultStatus,
		ErrorMessage:     errMsg,
		DurationMillis:   time.Since(start).Milliseconds(),
		RelatedIssueID:   issueID,
		RelatedTaskID:    taskID,
		RelatedDevTaskID: devTaskID,
	}
	if err := h.apiLogRepo.Create(log); err != nil {
		h.logger.Warn("记录API收发历史失败", zap.String("api_name", apiName), zap.Error(err))
	}
}

func readRequestBody(c *gin.Context) (string, error) {
	if c.Request == nil || c.Request.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body), nil
}

func bindJSONBody(c *gin.Context, rawBody string, target any) error {
	if c.Request != nil {
		c.Request.Body = io.NopCloser(strings.NewReader(rawBody))
	}
	return c.ShouldBindJSON(target)
}

func redactHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "key") || lower == "authorization" {
			result[key] = []string{"***"}
			continue
		}
		result[key] = values
	}
	return result
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
