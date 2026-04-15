package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type TestTaskHandler struct {
	genTestService      *service.GenTestService
	interventionService *service.InterventionService
	testTaskRepo        *repository.TestTaskRepo
	executionRepo       *repository.ExecutionRepo
	eventHub            *service.TaskEventHub
	logger              *zap.Logger
}

func NewTestTaskHandler(logger *zap.Logger) *TestTaskHandler {
	return &TestTaskHandler{
		genTestService:      service.NewGenTestService(logger),
		interventionService: service.NewInterventionService(logger),
		testTaskRepo:        repository.NewTestTaskRepo(),
		executionRepo:       repository.NewExecutionRepo(),
		eventHub:            service.DefaultTaskEventHub,
		logger:              logger,
	}
}

// Update 更新测试任务
// PUT /api/test-tasks/:id
func (h *TestTaskHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	task, err := h.testTaskRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	var req struct {
		Status       *string `json:"status"`
		RetryCount   *int    `json:"retry_count"`
		ErrorMessage *string `json:"error_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.RetryCount != nil {
		task.RetryCount = *req.RetryCount
	}
	if req.ErrorMessage != nil {
		task.ErrorMessage = *req.ErrorMessage
	}

	if err := h.testTaskRepo.Update(task); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, task)
}

// Delete 删除测试任务
// DELETE /api/test-tasks/:id
func (h *TestTaskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	task, err := h.testTaskRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	if task.Status == model.TaskStatusRunning {
		pkg.Fail(c, pkg.CodeParamError, "运行中的任务无法删除")
		return
	}

	if err := h.testTaskRepo.Delete(id); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// GenerateTest 重新生成测试(针对已有任务重新执行)
// POST /api/test-tasks/:id/generate
func (h *TestTaskHandler) GenerateTest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	task, err := h.testTaskRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	if task.Status == model.TaskStatusRunning {
		pkg.Fail(c, pkg.CodeParamError, "任务正在运行中，请勿重复生成")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := h.genTestService.RunTask(ctx, id); err != nil {
			h.logger.Error("重新生成测试失败", zap.Uint64("task_id", id), zap.Error(err))
			return
		}

		report, reportErr := h.genTestService.SelfTestTask(ctx, id)
		if reportErr != nil {
			h.logger.Error("重新生成自测失败", zap.Uint64("task_id", id), zap.Error(reportErr))
			return
		}
		if finalizeErr := h.genTestService.FinalizeTask(id, report); finalizeErr != nil {
			h.logger.Error("重新生成收尾失败", zap.Uint64("task_id", id), zap.Error(finalizeErr))
		}
	}()

	pkg.OK(c, gin.H{"task_id": id, "message": "已开始重新生成"})
}

// Publish 发布测试任务结果
// POST /api/test-tasks/:id/publish
func (h *TestTaskHandler) Publish(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	task, err := h.testTaskRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	if task.Status != model.TaskStatusCompleted {
		pkg.Fail(c, pkg.CodeParamError, "仅已完成状态的任务可以发布")
		return
	}

	pkg.OK(c, gin.H{"task_id": id, "message": "发布成功"})
}

// GetLogs 获取测试任务日志(非SSE，返回历史事件JSON)
// GET /api/test-tasks/:id/logs
func (h *TestTaskHandler) GetLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	if _, err := h.testTaskRepo.GetByID(id); err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	events, err := h.eventHub.GetLogsFromDB(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "获取运行日志失败")
		return
	}
	if events == nil {
		events = h.eventHub.GetHistory(id)
	}
	pkg.OK(c, events)
}

// List 测试任务列表
// GET /api/test-tasks
func (h *TestTaskHandler) List(c *gin.Context) {
	var query dto.TestTaskListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	offset := (query.Page - 1) * query.PageSize

	tasks, total, err := h.testTaskRepo.List(query.ProjectID, query.IssueID, query.Keyword, query.Status, offset, query.PageSize)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, tasks, total, query.Page, query.PageSize)
}

// GetByID 测试任务详情
// GET /api/test-tasks/:id
func (h *TestTaskHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	task, err := h.testTaskRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	pkg.OK(c, task)
}

// Create 手动创建测试任务(触发AI生成)
// POST /api/test-tasks
func (h *TestTaskHandler) Create(c *gin.Context) {
	var req dto.CreateTestTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	userID := middleware.GetCurrentUserID(c)
	task, err := h.genTestService.Execute(req.IssueID, req.AgentID, &userID, req.WorkflowName)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, task)
}

// GetTestCases 获取任务的测试用例列表
// GET /api/test-tasks/:id/cases
func (h *TestTaskHandler) GetTestCases(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	cases, err := h.testTaskRepo.GetTestCasesByTaskID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, cases)
}

// GetTestScripts 获取任务的测试脚本列表
// GET /api/test-tasks/:id/scripts
func (h *TestTaskHandler) GetTestScripts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	scripts, err := h.testTaskRepo.GetTestScriptsByTaskID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, service.NormalizeTestScripts(scripts))
}

// StreamEvents 测试任务事件流
// GET /api/test-tasks/:id/events
func (h *TestTaskHandler) StreamEvents(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	if _, err := h.testTaskRepo.GetByID(id); err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务不存在")
		return
	}

	history, ch, cancel := h.eventHub.Subscribe(id)
	defer cancel()

	w := c.Writer
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")

	c.Status(200)
	flusher, ok := w.(interface{ Flush() })
	if !ok {
		pkg.FailHTTP(c, 500, pkg.CodeInternalError, "streaming unsupported")
		return
	}

	for _, event := range history {
		if err := writeSSEEvent(w, "task-event", event); err != nil {
			return
		}
	}
	flusher.Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, "task-event", event); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w gin.ResponseWriter, eventName string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}

// ListTestCases 测试用例全局列表
// GET /api/test-cases
func (h *TestTaskHandler) ListTestCases(c *gin.Context) {
	var query dto.TestCaseListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	offset := (query.Page - 1) * query.PageSize

	cases, total, err := h.testTaskRepo.ListTestCases(
		query.ProjectID, query.IssueID, query.TaskID,
		query.Keyword, query.Category, query.Source, query.SelfTestResult,
		offset, query.PageSize,
	)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, cases, total, query.Page, query.PageSize)
}

// UpdateTestCase 人工修改测试用例
// PUT /api/test-cases/:id
func (h *TestTaskHandler) UpdateTestCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	var req dto.UpdateTestCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	operatorID := middleware.GetCurrentUserID(c)
	if err := h.interventionService.UpdateTestCase(id, operatorID, &req); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// UpdateTestScript 人工修改测试脚本
// PUT /api/test-scripts/:id
func (h *TestTaskHandler) UpdateTestScript(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	var req dto.UpdateTestScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	operatorID := middleware.GetCurrentUserID(c)
	if err := h.interventionService.UpdateTestScript(id, operatorID, &req); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// ListExecutions 测试执行记录列表
// GET /api/executions
func (h *TestTaskHandler) ListExecutions(c *gin.Context) {
	var query dto.ExecutionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	offset := (query.Page - 1) * query.PageSize

	execs, total, err := h.executionRepo.List(query.ProjectID, query.TaskID, query.Status, offset, query.PageSize)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, execs, total, query.Page, query.PageSize)
}

// GetInterventionHistory 获取问题单的人工介入记录
// GET /api/issues/:id/interventions
func (h *TestTaskHandler) GetInterventionHistory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	records, err := h.interventionService.GetInterventionHistory(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, records)
}
