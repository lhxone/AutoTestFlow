package model

import "time"

// GitCommitLog Git提交记录
type GitCommitLog struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ReviewTaskID  *uint64   `gorm:"index" json:"review_task_id"`
	TestTaskID    *uint64   `json:"test_task_id"`
	ProjectID     uint64    `gorm:"index;not null" json:"project_id"`
	CommitHash    string    `gorm:"size:64;index;default:''" json:"commit_hash"`
	Branch        string    `gorm:"size:128;default:''" json:"branch"`
	CommitMessage string    `gorm:"size:512;default:''" json:"commit_message"`
	FilesChanged  JSON      `gorm:"type:json" json:"files_changed"`
	PushStatus    string    `gorm:"size:16;default:'pending'" json:"push_status"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (GitCommitLog) TableName() string { return "git_commit_log" }
