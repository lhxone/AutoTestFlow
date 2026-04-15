package handler

import (
	"strconv"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{projectService: service.NewProjectService()}
}

// List 项目列表
// GET /api/projects
func (h *ProjectHandler) List(c *gin.Context) {
	var query dto.ProjectListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	projects, total, err := h.projectService.List(&query)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	pkg.OKPage(c, projects, total, query.Page, query.PageSize)
}

// Create 创建项目
// POST /api/projects
func (h *ProjectHandler) Create(c *gin.Context) {
	var req dto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	project, err := h.projectService.Create(&req)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, project)
}

// GetByID 项目详情
// GET /api/projects/:id
func (h *ProjectHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的项目ID")
		return
	}

	project, err := h.projectService.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "项目不存在")
		return
	}

	pkg.OK(c, project)
}

// Update 更新项目
// PUT /api/projects/:id
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的项目ID")
		return
	}

	var req dto.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	project, err := h.projectService.Update(id, &req)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, project)
}

// Delete 删除项目
// DELETE /api/projects/:id
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的项目ID")
		return
	}

	if err := h.projectService.Delete(id); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// ListIssueSyncLogs 项目问题单采集记录
// GET /api/projects/:id/issue-sync-logs
func (h *ProjectHandler) ListIssueSyncLogs(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的项目ID")
		return
	}

	var query dto.ProjectIssueSyncLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	logs, total, err := h.projectService.ListIssueSyncLogs(projectID, &query)
	if err != nil {
		if err.Error() == "项目不存在" {
			pkg.Fail(c, pkg.CodeNotFound, err.Error())
			return
		}
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	pkg.OKPage(c, logs, total, query.Page, query.PageSize)
}

// GetIssueSyncLogDetail 项目问题单采集详情（支持分页）
// GET /api/projects/:id/issue-sync-logs/:logId
func (h *ProjectHandler) GetIssueSyncLogDetail(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的项目ID")
		return
	}

	logID, err := strconv.ParseUint(c.Param("logId"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的采集记录ID")
		return
	}

	var query dto.IssueSyncLogDetailQuery
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

	log, details, total, err := h.projectService.GetIssueSyncLogDetailPaginated(projectID, logID, query.Page, query.PageSize)
	if err != nil {
		if err.Error() == "项目不存在" || err.Error() == "采集记录不存在" {
			pkg.Fail(c, pkg.CodeNotFound, err.Error())
			return
		}
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	type issueSyncLogDetailVO struct {
		ID            uint64                       `json:"id"`
		SyncLogID     uint64                       `json:"sync_log_id"`
		ProjectID     uint64                       `json:"project_id"`
		IssueID       *uint64                      `json:"issue_id"`
		ZentaoID      int                          `json:"zentao_id"`
		IssueTitle    string                       `json:"issue_title"`
		Action        string                       `json:"action"`
		ChangedFields []model.IssueSyncFieldChange `json:"changed_fields"`
		CreatedAt     any                          `json:"created_at"`
	}

	items := make([]issueSyncLogDetailVO, 0, len(details))
	for _, detail := range details {
		items = append(items, issueSyncLogDetailVO{
			ID:            detail.ID,
			SyncLogID:     detail.SyncLogID,
			ProjectID:     detail.ProjectID,
			IssueID:       detail.IssueID,
			ZentaoID:      detail.ZentaoID,
			IssueTitle:    detail.IssueTitle,
			Action:        detail.Action,
			ChangedFields: model.DecodeIssueSyncFieldChanges(detail.ChangedFieldsJSON),
			CreatedAt:     detail.CreatedAt,
		})
	}

	pkg.OK(c, gin.H{
		"log":         log,
		"details":     items,
		"total":       total,
		"page":        query.Page,
		"page_size":   query.PageSize,
	})
}

// ListAllIssueSyncLogs 全局采集记录列表
// GET /api/issue-sync-logs
func (h *ProjectHandler) ListAllIssueSyncLogs(c *gin.Context) {
	var query dto.IssueSyncLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	logs, total, err := h.projectService.ListAllIssueSyncLogs(&query)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	pkg.OKPage(c, logs, total, query.Page, query.PageSize)
}

// GetIssueSyncLogDetailByID 全局采集记录详情（支持分页）
// GET /api/issue-sync-logs/:logId
func (h *ProjectHandler) GetIssueSyncLogDetailByID(c *gin.Context) {
	logID, err := strconv.ParseUint(c.Param("logId"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的采集记录ID")
		return
	}

	var query dto.IssueSyncLogDetailQuery
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

	log, details, total, err := h.projectService.GetIssueSyncLogDetailByIDPaginated(logID, query.Page, query.PageSize)
	if err != nil {
		if err.Error() == "采集记录不存在" {
			pkg.Fail(c, pkg.CodeNotFound, err.Error())
			return
		}
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	type issueSyncLogDetailVO struct {
		ID            uint64                       `json:"id"`
		SyncLogID     uint64                       `json:"sync_log_id"`
		ProjectID     uint64                       `json:"project_id"`
		IssueID       *uint64                      `json:"issue_id"`
		ZentaoID      int                          `json:"zentao_id"`
		IssueTitle    string                       `json:"issue_title"`
		Action        string                       `json:"action"`
		ChangedFields []model.IssueSyncFieldChange `json:"changed_fields"`
		CreatedAt     any                          `json:"created_at"`
	}

	items := make([]issueSyncLogDetailVO, 0, len(details))
	for _, detail := range details {
		items = append(items, issueSyncLogDetailVO{
			ID:            detail.ID,
			SyncLogID:     detail.SyncLogID,
			ProjectID:     detail.ProjectID,
			IssueID:       detail.IssueID,
			ZentaoID:      detail.ZentaoID,
			IssueTitle:    detail.IssueTitle,
			Action:        detail.Action,
			ChangedFields: model.DecodeIssueSyncFieldChanges(detail.ChangedFieldsJSON),
			CreatedAt:     detail.CreatedAt,
		})
	}

	pkg.OK(c, gin.H{
		"log":       log,
		"details":   items,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	})
}
