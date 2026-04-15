package handler

import (
	"time"

	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CICallbackHandler struct {
	executionRepo *repository.ExecutionRepo
	issueRepo     *repository.IssueRepo
	logger        *zap.Logger
}

func NewCICallbackHandler(logger *zap.Logger) *CICallbackHandler {
	return &CICallbackHandler{
		executionRepo: repository.NewExecutionRepo(),
		issueRepo:     repository.NewIssueRepo(),
		logger:        logger,
	}
}

// CICallbackRequest CI 回调请求(pytest-json-report 格式简化)
type CICallbackRequest struct {
	ExecutionID uint64 `json:"execution_id"`
	Summary     struct {
		Total   int `json:"total"`
		Passed  int `json:"passed"`
		Failed  int `json:"failed"`
		Skipped int `json:"skipped"`
	} `json:"summary"`
	Duration float64 `json:"duration"`
	Tests    []struct {
		NodeID  string `json:"nodeid"`
		Outcome string `json:"outcome"` // passed/failed/skipped
		Message string `json:"message"`
	} `json:"tests"`
}

// Callback CI 测试结果回调
// POST /api/ci/callback
func (h *CICallbackHandler) Callback(c *gin.Context) {
	var req CICallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	if req.ExecutionID == 0 {
		// 如果没有指定 execution_id，尝试从最近的 pending 执行中匹配
		h.logger.Warn("CI回调未指定execution_id，跳过结果记录")
		pkg.OK(c, gin.H{"message": "received but no execution_id"})
		return
	}

	exec, err := h.executionRepo.GetByID(req.ExecutionID)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "执行记录不存在")
		return
	}

	// 更新执行记录
	now := time.Now()
	exec.TotalCases = req.Summary.Total
	exec.PassedCases = req.Summary.Passed
	exec.FailedCases = req.Summary.Failed
	exec.SkippedCases = req.Summary.Skipped
	exec.DurationSec = int(req.Duration)
	exec.CompletedAt = &now

	if req.Summary.Total > 0 {
		exec.PassRate = float64(req.Summary.Passed) / float64(req.Summary.Total) * 100
	}

	if req.Summary.Failed > 0 {
		exec.Status = "failed"
	} else if req.Summary.Total == req.Summary.Passed {
		exec.Status = "passed"
	} else {
		exec.Status = "passed" // skipped 不算失败
	}

	_ = h.executionRepo.Update(exec)

	h.logger.Info("CI回调处理完成",
		zap.Uint64("execution_id", exec.ID),
		zap.String("status", exec.Status),
		zap.Float64("pass_rate", exec.PassRate))

	pkg.OK(c, gin.H{"status": exec.Status, "pass_rate": exec.PassRate})
}
