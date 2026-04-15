package handler

import (
	"encoding/json"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{authService: service.NewAuthService()}
}

// Login 登录
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	resp, err := h.authService.Login(&req)
	if err != nil {
		h.writeAuthLog(c, nil, req.Username, "login_failed", gin.H{
			"result": "failed",
			"reason": err.Error(),
		})
		pkg.Fail(c, pkg.CodeUnauthorized, err.Error())
		return
	}

	h.writeAuthLog(c, &resp.User.ID, resp.User.Username, "login_success", gin.H{
		"result": "success",
	})
	pkg.OK(c, resp)
}

// RefreshToken 刷新令牌
// POST /api/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	resp, err := h.authService.RefreshToken(&req)
	if err != nil {
		pkg.Fail(c, pkg.CodeUnauthorized, err.Error())
		return
	}

	pkg.OK(c, resp)
}

// GetCurrentUser 获取当前用户信息
// GET /api/auth/me
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	info, err := h.authService.GetCurrentUser(userID)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, err.Error())
		return
	}
	pkg.OK(c, info)
}

// ChangePassword 修改密码
// PUT /api/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	userID := middleware.GetCurrentUserID(c)
	if err := h.authService.ChangePassword(userID, &req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// Logout 登出(前端清除token即可，后端仅记录日志)
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	username := middleware.GetCurrentUsername(c)
	h.writeAuthLog(c, &userID, username, "logout", gin.H{
		"result": "success",
	})
	pkg.OK(c, nil)
}

func (h *AuthHandler) writeAuthLog(c *gin.Context, userID *uint64, username, action string, detail gin.H) {
	payload, _ := json.Marshal(detail)
	_ = h.authService.CreateAuthLog(&model.OperationLog{
		UserID:     userID,
		Username:   username,
		Module:     "auth",
		Action:     action,
		TargetType: "user",
		TargetID:   userID,
		Detail:     model.JSON(payload),
		IP:         c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	})
}
