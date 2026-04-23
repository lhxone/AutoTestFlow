package model

import (
	"encoding/json"
	"time"
)

const (
	IssueSyncStatusRunning = "running"
	IssueSyncStatusSuccess = "success"
	IssueSyncStatusFailed  = "failed"

	IssueSyncActionAdded   = "added"
	IssueSyncActionUpdated = "updated"
	IssueSyncActionDeleted = "deleted"

	// 同步类型
	SyncTypeIssue    = "issue"    // Bug同步
	SyncTypeTestCase = "testcase" // 用例同步
)

// IssueSyncLog 问题单同步日志
type IssueSyncLog struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID    uint64     `gorm:"index;not null" json:"project_id"`
	SyncType     string     `gorm:"size:16;index;not null;default:'issue'" json:"sync_type"` // 同步类型: issue/testcase
	Status       string     `gorm:"size:16;index;not null;default:'running'" json:"status"`
	FullSync     bool       `gorm:"default:false" json:"full_sync"`
	AddedCount   int        `gorm:"default:0" json:"added_count"`
	UpdatedCount int        `gorm:"default:0" json:"updated_count"`
	DeletedCount int        `gorm:"default:0" json:"deleted_count"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	StartedAt    time.Time  `gorm:"autoCreateTime" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

func (IssueSyncLog) TableName() string { return "issue_sync_log" }

// IssueSyncFieldChange 问题单同步字段变更
type IssueSyncFieldChange struct {
	Field      string `json:"field"`
	FieldLabel string `json:"field_label"`
	OldValue   string `json:"old_value"`
	NewValue   string `json:"new_value"`
}

// IssueSyncLogDetail 问题单同步明细
type IssueSyncLogDetail struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncLogID         uint64    `gorm:"index;not null" json:"sync_log_id"`
	ProjectID         uint64    `gorm:"index;not null" json:"project_id"`
	SyncType          string    `gorm:"size:16;index;not null;default:'issue'" json:"sync_type"` // 同步类型
	IssueID           *uint64   `gorm:"index" json:"issue_id"`
	TestCaseID        *uint64   `gorm:"index" json:"test_case_id"` // 用例ID
	ZentaoID          int       `gorm:"index;not null;default:0" json:"zentao_id"`
	IssueTitle        string    `gorm:"size:512;not null;default:''" json:"issue_title"`
	Action            string    `gorm:"size:16;index;not null" json:"action"`
	ChangedFieldsJSON string    `gorm:"type:json" json:"-"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (IssueSyncLogDetail) TableName() string { return "issue_sync_log_detail" }

func EncodeIssueSyncFieldChanges(changes []IssueSyncFieldChange) string {
	if len(changes) == 0 {
		return "[]"
	}

	data, err := json.Marshal(changes)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func DecodeIssueSyncFieldChanges(raw string) []IssueSyncFieldChange {
	if raw == "" {
		return []IssueSyncFieldChange{}
	}

	var changes []IssueSyncFieldChange
	if err := json.Unmarshal([]byte(raw), &changes); err != nil {
		return []IssueSyncFieldChange{}
	}
	return changes
}
