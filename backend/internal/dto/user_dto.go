package dto

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string   `json:"username" binding:"required,min=2,max=64"`
	Password string   `json:"password" binding:"required,min=6,max=128"`
	RealName string   `json:"real_name" binding:"max=64"`
	Email    string   `json:"email" binding:"omitempty,email,max=128"`
	Phone    string   `json:"phone" binding:"max=20"`
	RoleIDs  []uint64 `json:"role_ids" binding:"required,min=1"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	RealName string   `json:"real_name" binding:"max=64"`
	Email    string   `json:"email" binding:"omitempty,email,max=128"`
	Phone    string   `json:"phone" binding:"max=20"`
	Status   *int8    `json:"status" binding:"omitempty,oneof=0 1"`
	RoleIDs  []uint64 `json:"role_ids"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

// UserListQuery 用户列表查询
type UserListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   *int8  `form:"status"`
	RoleCode string `form:"role_code"`
}

type LoginLogListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Username string `form:"username"`
	Action   string `form:"action"`
}
