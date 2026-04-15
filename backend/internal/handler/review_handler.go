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

type ReviewHandler struct {
	reviewService *service.ReviewService
	reviewRepo    *repository.ReviewRepo
}

func NewReviewHandler(logger *zap.Logger) *ReviewHandler {
	return &ReviewHandler{
		reviewService: service.NewReviewService(logger),
		reviewRepo:    repository.NewReviewRepo(),
	}
}

// List Review列表
// GET /api/reviews
func (h *ReviewHandler) List(c *gin.Context) {
	var query dto.ReviewListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	tasks, total, err := h.reviewService.List(&query)
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
	pkg.OKPage(c, tasks, total, query.Page, query.PageSize)
}

// GetDetail Review详情
// GET /api/reviews/:id
func (h *ReviewHandler) GetDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	detail, err := h.reviewService.GetDetail(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, err.Error())
		return
	}

	pkg.OK(c, detail)
}

// DoReview 执行审核
// POST /api/reviews/:id/review
func (h *ReviewHandler) DoReview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	var req dto.ReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	reviewerID := middleware.GetCurrentUserID(c)
	if err := h.reviewService.DoReview(id, reviewerID, &req); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}
