package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/config"
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

// GetSelfTestReport 获取任务自测报告内容
// GET /api/test-tasks/:id/self-test-report?framework=playwright|midscene
func (h *TestTaskHandler) GetSelfTestReport(c *gin.Context) {
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

	framework := strings.ToLower(strings.TrimSpace(c.Query("framework")))
	if framework != "playwright" {
		pkg.Fail(c, pkg.CodeParamError, "framework 仅支持 playwright")
		return
	}

	var output map[string]any
	if len(task.AIOutput) == 0 || json.Unmarshal(task.AIOutput, &output) != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务未产出自测报告")
		return
	}

	selfTest, ok := output["self_test"].(map[string]any)
	if !ok {
		pkg.Fail(c, pkg.CodeNotFound, "任务未产出自测报告")
		return
	}

	reportPath := extractFrameworkReportPath(selfTest)
	if strings.TrimSpace(reportPath) == "" {
		// Fallback：旧任务可能没有 report_path，尝试默认路径
		reportPath = defaultPlaywrightReportPath()
	}

	repoDir := ""
	if workspace, ok := output["workspace"].(map[string]any); ok {
		repoDir, _ = workspace["repo_dir"].(string)
		repoDir = strings.TrimSpace(repoDir)
	}
	if repoDir == "" {
		repoDir = filepath.Join(config.Global.Git.WorkDir, "cli-runtime", fmt.Sprintf("project_%d", task.ProjectID), fmt.Sprintf("task_%d", task.ID), "repo")
	}

	normalizedPath := normalizeReportRelativePath(reportPath)
	content, contentType, readErr := readTaskReportFile(repoDir, normalizedPath)
	if readErr != nil {
		pkg.Fail(c, pkg.CodeNotFound, readErr.Error())
		return
	}

	// 内嵌报告中的资源引用（视频、图片、trace 等）为 base64 data URI
	if contentType == "text/html" {
		content = embedReportAssets(repoDir, content)
	}

	pkg.OK(c, gin.H{
		"framework":    framework,
		"report_path":  normalizedPath,
		"content":      content,
		"content_type": contentType,
	})
}

// GetWorkspaceFile 获取任务工作区文件
// GET /api/test-tasks/:id/workspace/*filepath
func (h *TestTaskHandler) GetWorkspaceFile(c *gin.Context) {
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

	var output map[string]any
	if len(task.AIOutput) == 0 || json.Unmarshal(task.AIOutput, &output) != nil {
		pkg.Fail(c, pkg.CodeNotFound, "任务无工作区")
		return
	}

	repoDir := ""
	if workspace, ok := output["workspace"].(map[string]any); ok {
		repoDir, _ = workspace["repo_dir"].(string)
		repoDir = strings.TrimSpace(repoDir)
	}
	if repoDir == "" {
		repoDir = filepath.Join(config.Global.Git.WorkDir, "cli-runtime", fmt.Sprintf("project_%d", task.ProjectID), fmt.Sprintf("task_%d", task.ID), "repo")
	}

	cleanRepo, err := filepath.Abs(repoDir)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "解析工作区路径失败")
		return
	}

	relativePath := strings.TrimPrefix(c.Param("filepath"), "/")
	if relativePath == "" {
		pkg.Fail(c, pkg.CodeParamError, "文件路径不能为空")
		return
	}

	targetPath := filepath.Join(cleanRepo, filepath.FromSlash(relativePath))
	cleanTarget, err := filepath.Abs(targetPath)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "解析文件路径失败")
		return
	}

	repoPrefix := cleanRepo + string(filepath.Separator)
	if cleanTarget != cleanRepo && !strings.HasPrefix(cleanTarget, repoPrefix) {
		pkg.Fail(c, pkg.CodeForbidden, "非法的文件路径")
		return
	}

	data, err := os.ReadFile(cleanTarget)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "文件不存在")
		return
	}

	contentType := detectReportContentType(cleanTarget, data)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, contentType, data)
}

// assetMatcher 匹配 HTML 中的相对资源引用（src 和 href 属性）
var assetMatcher = regexp.MustCompile(`(src|href)=["']((?:data|trace|resources|blob)[^"']*?\.(?:webm|png|jpg|jpeg|gif|svg|json|trace|css))["']`)

// embedReportAssets 将 Playwright 报告中的相对资源路径替换为 base64 data URI。
func embedReportAssets(repoDir, html string) string {
	reportDir := filepath.Join(repoDir, filepath.FromSlash("playwright-report"))

	return assetMatcher.ReplaceAllStringFunc(html, func(match string) string {
		submatches := assetMatcher.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		attr := submatches[1]
		relPath := submatches[2]

		// 安全检查：防止路径遍历
		cleanRel := filepath.ToSlash(filepath.Clean(relPath))
		if strings.HasPrefix(cleanRel, "..") {
			return match
		}

		fullPath := filepath.Join(reportDir, filepath.FromSlash(cleanRel))
		cleanFull, err := filepath.Abs(fullPath)
		if err != nil {
			return match
		}

		cleanReport, err := filepath.Abs(reportDir)
		if err != nil {
			return match
		}
		prefix := cleanReport + string(filepath.Separator)
		if !strings.HasPrefix(cleanFull, prefix) && cleanFull != cleanReport {
			return match
		}

		data, err := os.ReadFile(cleanFull)
		if err != nil {
			return match
		}

		mimeType := detectReportContentType(cleanFull, data)
		encoded := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf(`%s="data:%s;base64,%s"`, attr, mimeType, encoded)
	})
}

func extractFrameworkReportPath(report map[string]any) string {
	raw, ok := report["playwright"]
	if !ok {
		return ""
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	path, _ := obj["report_path"].(string)
	return strings.TrimSpace(path)
}

func defaultPlaywrightReportPath() string {
	return "playwright-report/index.html"
}

func normalizeReportRelativePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, ".\\")
	return filepath.ToSlash(path)
}

func readTaskReportFile(repoDir, relativePath string) (string, string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", "", fmt.Errorf("报告路径为空")
	}

	cleanRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", "", fmt.Errorf("解析报告目录失败: %w", err)
	}

	targetPath := filepath.Join(cleanRepo, filepath.FromSlash(relativePath))
	cleanTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", "", fmt.Errorf("解析报告文件路径失败: %w", err)
	}

	repoPrefix := cleanRepo + string(filepath.Separator)
	if cleanTarget != cleanRepo && !strings.HasPrefix(cleanTarget, repoPrefix) {
		return "", "", fmt.Errorf("报告路径非法")
	}

	data, err := os.ReadFile(cleanTarget)
	if err != nil {
		return "", "", fmt.Errorf("读取报告文件失败: %w", err)
	}

	contentType := detectReportContentType(cleanTarget, data)
	return string(data), contentType, nil
}

func detectReportContentType(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".txt", ".log":
		return "text/plain"
	}
	if len(data) == 0 {
		return "text/plain"
	}
	return http.DetectContentType(data)
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
