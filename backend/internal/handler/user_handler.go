package handler

import (
	"strconv"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{userService: service.NewUserService()}
}

// List 用户列表
// GET /api/users
func (h *UserHandler) List(c *gin.Context) {
	var query dto.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	users, total, err := h.userService.List(&query)
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
	pkg.OKPage(c, users, total, query.Page, query.PageSize)
}

// ListLoginLogs 登录日志列表
// GET /api/users/login-logs
func (h *UserHandler) ListLoginLogs(c *gin.Context) {
	var query dto.LoginLogListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	logs, total, err := h.userService.ListLoginLogs(&query)
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
	pkg.OKPage(c, logs, total, query.Page, query.PageSize)
}

// Create 创建用户
// POST /api/users
func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	user, err := h.userService.Create(&req)
	if err != nil {
		pkg.Fail(c, pkg.CodeDuplicate, err.Error())
		return
	}

	pkg.OK(c, user)
}

// GetByID 用户详情
// GET /api/users/:id
func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的用户ID")
		return
	}

	user, err := h.userService.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "用户不存在")
		return
	}

	pkg.OK(c, user)
}

// Update 更新用户
// PUT /api/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的用户ID")
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	user, err := h.userService.Update(id, &req)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, user)
}

// Delete 删除用户
// DELETE /api/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的用户ID")
		return
	}

	if err := h.userService.Delete(id); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}
