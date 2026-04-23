package handler

import (
	"strconv"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ZentaoTestCaseHandler struct {
	syncService *service.ZentaoTestCaseSyncService
	genService  *service.ZentaoTestCaseGenService
	tcRepo      *repository.ZentaoTestCaseRepo
}

func NewZentaoTestCaseHandler(logger *zap.Logger) *ZentaoTestCaseHandler {
	return &ZentaoTestCaseHandler{
		syncService: service.NewZentaoTestCaseSyncService(logger),
		genService:  service.NewZentaoTestCaseGenService(logger),
		tcRepo:      repository.NewZentaoTestCaseRepo(),
	}
}

// List 用例列表
// GET /api/test-cases/zentao
func (h *ZentaoTestCaseHandler) List(c *gin.Context) {
	var query dto.ZentaoTestCaseListQuery
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

	projectID := query.ProjectID
	if projectID == 0 {
		projectID = query.ProductID
	}

	cases, total, err := h.tcRepo.List(projectID, query.ProductID, query.TestStatus, query.Branch, query.Keyword, query.Type, offset, query.PageSize)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, cases, total, query.Page, query.PageSize)
}

// Sync 手动触发同步用例
// POST /api/test-cases/zentao/sync
func (h *ZentaoTestCaseHandler) Sync(c *gin.Context) {
	var req dto.SyncTestCasesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	result, err := h.syncService.SyncTestCases(req.ProjectID, req.FullSync)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "同步失败: "+err.Error())
		return
	}

	pkg.OK(c, gin.H{
		"message":    "用例同步完成",
		"synced":     result.SyncedCount,
		"added":      result.AddedCount,
		"updated":    result.UpdatedCount,
		"deleted":    result.DeletedCount,
	})
}

// GenerateScript 根据用例生成测试脚本
// POST /api/test-cases/zentao/generate
func (h *ZentaoTestCaseHandler) GenerateScript(c *gin.Context) {
	var req dto.GenerateTestScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	userID := middleware.GetCurrentUserID(c)

	task, err := h.genService.GenerateScript(req.TestCaseID, req.AgentID, &userID)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "生成失败: "+err.Error())
		return
	}

	pkg.OK(c, gin.H{
		"message":  "脚本生成任务已创建",
		"task_id":  task.ID,
		"status":   task.Status,
	})
}

// GetByID 用例详情
// GET /api/test-cases/zentao/:id
func (h *ZentaoTestCaseHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	tc, err := h.tcRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "用例不存在")
		return
	}

	pkg.OK(c, tc)
}
