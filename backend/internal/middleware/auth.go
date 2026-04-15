package middleware

import (
	"strings"

	"auto-test-flow/internal/pkg"

	"github.com/gin-gonic/gin"
)

const (
	HeaderAuthorization = "Authorization"
	ContextUserID       = "user_id"
	ContextUsername     = "username"
	ContextRoleCode     = "role_code"
)

// JWTAuth JWT认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractBearerToken(c)
		if err != nil {
			pkg.Unauthorized(c, err.Error())
			return
		}
		if tokenString == "" {
			pkg.Unauthorized(c, "缺少认证令牌")
			return
		}

		claims, err := pkg.ParseToken(tokenString)
		if err != nil {
			pkg.Unauthorized(c, "认证令牌无效或已过期")
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUsername, claims.Username)
		c.Set(ContextRoleCode, claims.RoleCode)

		c.Next()
	}
}

func extractBearerToken(c *gin.Context) (string, error) {
	authHeader := strings.TrimSpace(c.GetHeader(HeaderAuthorization))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return "", pkgError("认证令牌格式错误")
		}
		return strings.TrimSpace(parts[1]), nil
	}

	queryToken := strings.TrimSpace(c.Query("access_token"))
	if queryToken != "" {
		return queryToken, nil
	}
	return "", nil
}

type pkgError string

func (e pkgError) Error() string { return string(e) }

// GetCurrentUserID 从上下文获取当前用户ID
func GetCurrentUserID(c *gin.Context) uint64 {
	v, exists := c.Get(ContextUserID)
	if !exists {
		return 0
	}
	return v.(uint64)
}

// GetCurrentUsername 从上下文获取当前用户名
func GetCurrentUsername(c *gin.Context) string {
	v, exists := c.Get(ContextUsername)
	if !exists {
		return ""
	}
	return v.(string)
}

// GetCurrentRoleCode 从上下文获取当前角色编码
func GetCurrentRoleCode(c *gin.Context) string {
	v, exists := c.Get(ContextRoleCode)
	if !exists {
		return ""
	}
	return v.(string)
}
