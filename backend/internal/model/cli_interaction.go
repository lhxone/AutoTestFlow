package model

import "time"

// CLIInteraction CLI交互记录表（AI提问、权限请求等）
type CLIInteraction struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	TaskID          uint       `gorm:"not null;index:idx_task_id" json:"task_id"`
	InteractionType string     `gorm:"size:32;not null;index:idx_type" json:"interaction_type"` // ai_question/permission_request
	Content         string     `gorm:"type:text;not null" json:"content"`
	Metadata        JSON       `gorm:"type:json" json:"metadata"`
	Status          string     `gorm:"size:16;not null;default:pending;index:idx_status" json:"status"` // pending/approved/rejected/answered
	UserResponse    string     `gorm:"type:text" json:"user_response"`
	UserID          uint       `gorm:"index" json:"user_id"`
	RespondedAt     *time.Time `json:"responded_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (CLIInteraction) TableName() string {
	return "cli_interaction"
}
