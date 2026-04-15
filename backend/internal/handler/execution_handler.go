package handler

import (
	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ExecutionHandler struct {
	ciService *service.CIService
}

func NewExecutionHandler(logger *zap.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		ciService: service.NewCIService(logger),
	}
}

// TriggerExecution 手动触发测试执行
// POST /api/executions/trigger
func (h *ExecutionHandler) TriggerExecution(c *gin.Context) {
	var req dto.TriggerExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	userID := middleware.GetCurrentUserID(c)
	exec, err := h.ciService.TriggerExecution(req.ProjectID, req.Branch, &userID)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, exec)
}
