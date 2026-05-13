package handler

import (
	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SettingHandler struct {
	settingService *service.SettingService
}

func NewSettingHandler(logger *zap.Logger) *SettingHandler {
	return &SettingHandler{
		settingService: service.NewSettingService(logger),
	}
}

// GetZentaoSettings 获取禅道配置
// GET /api/settings/zentao
func (h *SettingHandler) GetZentaoSettings(c *gin.Context) {
	settings, err := h.settingService.GetSettings("zentao")
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, settings)
}

// SaveZentaoSettings 保存禅道配置
// PUT /api/settings/zentao
func (h *SettingHandler) SaveZentaoSettings(c *gin.Context) {
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	userID := middleware.GetCurrentUserID(c)
	if err := h.settingService.SaveSettings("zentao", &req, userID); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// TestZentaoConnection 测试禅道连接
// POST /api/settings/zentao/test
func (h *SettingHandler) TestZentaoConnection(c *gin.Context) {
	var req dto.ZentaoTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	result := h.settingService.TestZentaoConnection(&req)
	pkg.OK(c, result)
}

// GetGitLabSettings 获取GitLab配置
// GET /api/settings/gitlab
func (h *SettingHandler) GetGitLabSettings(c *gin.Context) {
	settings, err := h.settingService.GetSettings("gitlab")
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, settings)
}

// SaveGitLabSettings 保存GitLab配置
// PUT /api/settings/gitlab
func (h *SettingHandler) SaveGitLabSettings(c *gin.Context) {
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	userID := middleware.GetCurrentUserID(c)
	if err := h.settingService.SaveSettings("gitlab", &req, userID); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// TestGitLabConnection 测试GitLab连接
// POST /api/settings/gitlab/test
func (h *SettingHandler) TestGitLabConnection(c *gin.Context) {
	var req dto.GitLabTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	result := h.settingService.TestGitLabConnection(&req)
	pkg.OK(c, result)
}

// GetMailSettings 获取邮件配置
// GET /api/settings/mail
func (h *SettingHandler) GetMailSettings(c *gin.Context) {
	settings, err := h.settingService.GetSettings("mail")
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, settings)
}

// SaveMailSettings 保存邮件配置
// PUT /api/settings/mail
func (h *SettingHandler) SaveMailSettings(c *gin.Context) {
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	userID := middleware.GetCurrentUserID(c)
	if err := h.settingService.SaveSettings("mail", &req, userID); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// TestMailConnection 测试邮件配置
// POST /api/settings/mail/test
func (h *SettingHandler) TestMailConnection(c *gin.Context) {
	var req dto.MailTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	result := h.settingService.TestMailConnection(&req)
	pkg.OK(c, result)
}

// SendTestMail 发送测试邮件
// POST /api/settings/mail/send-test
func (h *SettingHandler) SendTestMail(c *gin.Context) {
	var req dto.SendTestMailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	result := h.settingService.SendTestMail(&req)
	pkg.OK(c, result)
}

// GetCLIRuntimeSettings 获取 CLI Runtime 配置
// GET /api/settings/cli-runtime
func (h *SettingHandler) GetCLIRuntimeSettings(c *gin.Context) {
	settings, err := h.settingService.GetSettings("cli_runtime")
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, settings)
}

// SaveCLIRuntimeSettings 保存 CLI Runtime 配置
// PUT /api/settings/cli-runtime
func (h *SettingHandler) SaveCLIRuntimeSettings(c *gin.Context) {
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	userID := middleware.GetCurrentUserID(c)
	if err := h.settingService.SaveSettings("cli_runtime", &req, userID); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// GetRuntimeSettings 获取运行时配置
// GET /api/settings/runtime
func (h *SettingHandler) GetRuntimeSettings(c *gin.Context) {
	settings, err := h.settingService.GetSettings("runtime")
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, settings)
}

// SaveRuntimeSettings 保存运行时配置
// PUT /api/settings/runtime
func (h *SettingHandler) SaveRuntimeSettings(c *gin.Context) {
	var req dto.SaveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	userID := middleware.GetCurrentUserID(c)
	if err := h.settingService.SaveSettings("runtime", &req, userID); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}
