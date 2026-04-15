package middleware

import (
	"auto-test-flow/internal/pkg"

	"github.com/gin-gonic/gin"
)

// RequirePermission 权限检查中间件
// 检查当前用户是否拥有指定权限(基于角色)
func RequirePermission(permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleCode := GetCurrentRoleCode(c)

		// admin 角色拥有全部权限
		if roleCode == "admin" {
			c.Next()
			return
		}

		// 从数据库查询角色是否拥有该权限
		// 这里通过全局的权限缓存来判断，避免每次请求查数据库
		if !hasPermission(roleCode, permCode) {
			pkg.Forbidden(c, "无操作权限")
			return
		}

		c.Next()
	}
}

// RequireRoles 角色检查中间件
func RequireRoles(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		roleCode := GetCurrentRoleCode(c)
		if !roleSet[roleCode] {
			pkg.Forbidden(c, "角色无权访问")
			return
		}
		c.Next()
	}
}

// PermissionCache 权限缓存(角色code -> 权限code集合)
// 在应用启动时从数据库加载
var permissionCache map[string]map[string]bool

// InitPermissionCache 初始化权限缓存
func InitPermissionCache(cache map[string]map[string]bool) {
	permissionCache = cache
}

func hasPermission(roleCode, permCode string) bool {
	if permissionCache == nil {
		return false
	}
	perms, ok := permissionCache[roleCode]
	if !ok {
		return false
	}
	return perms[permCode]
}

// HasPermission 供非中间件场景复用权限判断逻辑
func HasPermission(roleCode, permCode string) bool {
	return hasPermission(roleCode, permCode)
}
