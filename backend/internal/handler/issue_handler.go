package handler

import (
	"fmt"
	"strconv"
	"strings"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IssueHandler struct {
	zentaoService *service.ZentaoService
	issueRepo     *repository.IssueRepo
	settingRepo   *repository.SettingRepo
}

func NewIssueHandler(logger *zap.Logger) *IssueHandler {
	return &IssueHandler{
		zentaoService: service.NewZentaoService(logger),
		issueRepo:     repository.NewIssueRepo(),
		settingRepo:   repository.NewSettingRepo(),
	}
}

// List 问题单列表
// GET /api/issues
func (h *IssueHandler) List(c *gin.Context) {
	var query dto.IssueListQuery
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
	projectID := query.ProjectID
	if projectID == 0 {
		projectID = query.LegacyProjectSetID
	}
	offset := (query.Page - 1) * query.PageSize

	issues, total, err := h.issueRepo.List(
		projectID, query.ZentaoStatus, query.TestStatus,
		query.Branch, query.Keyword, query.Assignee, offset, query.PageSize,
	)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, issues, total, query.Page, query.PageSize)
}

// GetByID 问题单详情
// GET /api/issues/:id
func (h *IssueHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	issue, err := h.issueRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "问题单不存在")
		return
	}

	resp := struct {
		model.Issue
		ZentaoURL string `json:"zentao_url"`
	}{
		Issue:     *issue,
		ZentaoURL: h.buildZentaoIssueURL(issue.ZentaoID),
	}

	pkg.OK(c, resp)
}

func (h *IssueHandler) buildZentaoIssueURL(zentaoID int) string {
	if zentaoID <= 0 {
		return ""
	}

	baseURL := strings.TrimSpace(h.settingRepo.GetValue("zentao", "base_url"))
	if baseURL == "" {
		return ""
	}

	base := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/bug-view-%d.html", base, zentaoID)
}

// Sync 手动触发同步（异步）
// POST /api/issues/sync
func (h *IssueHandler) Sync(c *gin.Context) {
	var req dto.SyncIssuesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	// 异步触发同步，立即返回 sync_log_id
	syncLogID := h.zentaoService.StartAsyncSync(req.ProjectID, req.FullSync)

	pkg.OK(c, gin.H{
		"message":     "同步任务已触发",
		"sync_log_id": syncLogID,
	})
}

// UpdateTestStatus 手动更新测试状态
// PUT /api/issues/:id/test-status
func (h *IssueHandler) UpdateTestStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	var req dto.UpdateTestStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	if err := h.issueRepo.ForceUpdateTestStatus(id, req.TestStatus); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	// 记录状态变更
	userID := middleware.GetCurrentUserID(c)
	_ = h.issueRepo.CreateStatusLog(&model.IssueStatusLog{
		IssueID:     id,
		Field:       "test_status",
		NewValue:    req.TestStatus,
		TriggerType: "manual",
		OperatorID:  &userID,
		Remark:      req.Remark,
	})

	pkg.OK(c, nil)
}