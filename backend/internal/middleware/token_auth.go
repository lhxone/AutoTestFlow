package middleware

import (
	"strings"

	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"

	"github.com/gin-gonic/gin"
)

const (
	HeaderATFToken = "X-ATF-Token"
)

// TokenAuth Token认证中间件（用于CI/CD集成接口）
func TokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader(HeaderATFToken))
		if token == "" {
			pkg.Unauthorized(c, "缺少认证令牌")
			return
		}

		// 验证Token
		settingRepo := repository.NewSettingRepo()
		storedToken := settingRepo.GetValue("integration", "api_token")
		if storedToken == "" {
			pkg.Unauthorized(c, "系统未配置API Token")
			return
		}

		if token != storedToken {
			pkg.Unauthorized(c, "认证令牌无效")
			return
		}

		c.Next()
	}
}
