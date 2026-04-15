package model

import "time"

// OperationLog 操作审计日志
type OperationLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     *uint64   `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:64;default:''" json:"username"`
	Module     string    `gorm:"size:32;index:idx_module_action;not null" json:"module"`
	Action     string    `gorm:"size:32;index:idx_module_action;not null" json:"action"`
	TargetType string    `gorm:"size:32;index:idx_target;default:''" json:"target_type"`
	TargetID   *uint64   `gorm:"index:idx_target" json:"target_id"`
	Detail     JSON      `gorm:"type:json" json:"detail"`
	IP         string    `gorm:"size:45;default:''" json:"ip"`
	UserAgent  string    `gorm:"size:256;default:''" json:"user_agent"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (OperationLog) TableName() string { return "operation_log" }
