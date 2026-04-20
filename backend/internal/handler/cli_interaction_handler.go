package handler

import (
	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/pkg"
	cliservice "auto-test-flow/internal/service"
	"go.uber.org/zap"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CLIInteractionHandler struct {
	service *cliservice.CLIInteractionService
}

func NewCLIInteractionHandler(service *cliservice.CLIInteractionService) *CLIInteractionHandler {
	if service == nil {
		logger, _ := zap.NewProduction()
		service = cliservice.NewCLIInteractionService(logger)
	}
	return &CLIInteractionHandler{service: service}
}

func (h *CLIInteractionHandler) RegisterRoutes(r *gin.RouterGroup) {
	interactions := r.Group("/:id/interactions")
	{
		interactions.GET("", h.GetByTaskID)
		interactions.GET("/pending", h.GetPendingByTaskID)
		interactions.POST("/:interactionId/reply", h.ReplyInteraction)
		interactions.POST("/:interactionId/approve", h.ApproveInteraction)
		interactions.POST("/:interactionId/reject", h.RejectInteraction)
	}
}

func (h *CLIInteractionHandler) GetByTaskID(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的任务ID")
		return
	}

	interactions, err := h.service.GetByTaskID(uint(taskID))
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "获取交互记录失败")
		return
	}

	responses := make([]dto.CLIInteractionResponse, len(interactions))
	for i, interaction := range interactions {
		responses[i] = dto.CLIInteractionResponse{
			ID:              interaction.ID,
			TaskID:          interaction.TaskID,
			InteractionType: interaction.InteractionType,
			Content:         interaction.Content,
			Metadata:        interaction.Metadata,
			Status:          interaction.Status,
			UserResponse:    interaction.UserResponse,
			UserID:          interaction.UserID,
			CreatedAt:       interaction.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:       interaction.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if interaction.RespondedAt != nil {
			responses[i].RespondedAt = interaction.RespondedAt.Format("2006-01-02 15:04:05")
		}
	}

	pkg.OK(c, responses)
}

func (h *CLIInteractionHandler) GetPendingByTaskID(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的任务ID")
		return
	}

	interactions, err := h.service.GetPendingByTaskID(uint(taskID))
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "获取待处理交互失败")
		return
	}

	responses := make([]dto.CLIInteractionResponse, len(interactions))
	for i, interaction := range interactions {
		responses[i] = dto.CLIInteractionResponse{
			ID:              interaction.ID,
			TaskID:          interaction.TaskID,
			InteractionType: interaction.InteractionType,
			Content:         interaction.Content,
			Metadata:        interaction.Metadata,
			Status:          interaction.Status,
			UserResponse:    interaction.UserResponse,
			UserID:          interaction.UserID,
			CreatedAt:       interaction.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:       interaction.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	pkg.OK(c, responses)
}

func (h *CLIInteractionHandler) ReplyInteraction(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("interactionId"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的交互ID")
		return
	}

	var req dto.ReplyInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的请求参数")
		return
	}

	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		pkg.Unauthorized(c, "用户未登录")
		return
	}
	if err := h.service.ReplyInteraction(uint(id), uint(userID), req.Response); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

func (h *CLIInteractionHandler) ApproveInteraction(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("interactionId"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的交互ID")
		return
	}

	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		pkg.Unauthorized(c, "用户未登录")
		return
	}
	if err := h.service.ApproveInteraction(uint(id), uint(userID)); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

func (h *CLIInteractionHandler) RejectInteraction(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("interactionId"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的交互ID")
		return
	}

	var req dto.ApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的请求参数")
		return
	}

	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		pkg.Unauthorized(c, "用户未登录")
		return
	}
	if err := h.service.RejectInteraction(uint(id), uint(userID), req.Reason); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}
