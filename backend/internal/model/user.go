package model

import "time"

// User 用户
type User struct {
	BaseModel
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	RealName     string     `gorm:"size:64;default:''" json:"real_name"`
	Email        string     `gorm:"size:128;index;default:''" json:"email"`
	Phone        string     `gorm:"size:20;default:''" json:"phone"`
	Avatar       string     `gorm:"size:512;default:''" json:"avatar"`
	Status       int8       `gorm:"default:1;not null" json:"status"` // 1=启用 0=禁用
	LastLoginAt  *time.Time `json:"last_login_at"`
	// 关联
	Roles []Role `gorm:"many2many:user_role;" json:"roles,omitempty"`
}

func (User) TableName() string { return "user" }

// Role 角色
type Role struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string `gorm:"size:32;uniqueIndex;not null" json:"code"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Description string `gorm:"size:255;default:''" json:"description"`
	// 关联
	Permissions []Permission `gorm:"many2many:role_permission;" json:"permissions,omitempty"`
}

func (Role) TableName() string { return "role" }

// Permission 权限
type Permission struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Module      string `gorm:"size:32;index;not null" json:"module"`
	Description string `gorm:"size:255;default:''" json:"description"`
}

func (Permission) TableName() string { return "permission" }

// UserRole 用户角色关联
type UserRole struct {
	ID     uint64 `gorm:"primaryKey;autoIncrement"`
	UserID uint64 `gorm:"uniqueIndex:uk_user_role;not null"`
	RoleID uint64 `gorm:"uniqueIndex:uk_user_role;index;not null"`
}

func (UserRole) TableName() string { return "user_role" }

// RolePermission 角色权限关联
type RolePermission struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	RoleID       uint64 `gorm:"uniqueIndex:uk_role_perm;not null"`
	PermissionID uint64 `gorm:"uniqueIndex:uk_role_perm;index;not null"`
}

func (RolePermission) TableName() string { return "role_permission" }
