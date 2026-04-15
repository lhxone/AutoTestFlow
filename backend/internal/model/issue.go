package model

import "time"

// 测试状态枚举
const (
	TestStatusPending                = "pending"
	TestStatusGenerating             = "generating"
	TestStatusReviewPending          = "review_pending"
	TestStatusReviewApproved         = "review_approved"
	TestStatusReviewRejected         = "review_rejected"
	TestStatusTesting                = "testing"
	TestStatusPassed                 = "passed"
	TestStatusPartialPassed          = "partial_passed"
	TestStatusAllFailed              = "all_failed"
	TestStatusInterventionNeeded     = "intervention_needed"
	TestStatusInterventionInProgress = "intervention_in_progress"
	TestStatusError                  = "error"
)

// Issue 问题单
type Issue struct {
	BaseModelNoSoftDelete
	ZentaoID        int       `gorm:"uniqueIndex:uk_zentao_project;not null" json:"zentao_id"`
	ProjectID       uint64    `gorm:"uniqueIndex:uk_zentao_project;index;not null" json:"project_id"`
	Title           string    `gorm:"size:512;not null" json:"title"`
	Description     string    `gorm:"type:text" json:"description"`
	IssueType       string    `gorm:"size:32;default:'bug'" json:"issue_type"`
	ZentaoStatus    string    `gorm:"size:32;index;default:''" json:"zentao_status"`
	TestStatus      string    `gorm:"size:32;index;default:'pending'" json:"test_status"`
	Severity        string    `gorm:"size:16;default:'normal'" json:"severity"`
	Priority        int8      `gorm:"default:3" json:"priority"`
	Reporter        string    `gorm:"size:64;default:''" json:"reporter"`
	ReporterEmail   string    `gorm:"size:128;default:''" json:"reporter_email"`
	Assignee        string    `gorm:"size:64;index;default:''" json:"assignee"`
	AssigneeEmail   string    `gorm:"size:128;default:''" json:"assignee_email"`
	Branch          string    `gorm:"size:128;default:''" json:"branch"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	ZentaoUpdatedAt *time.Time `json:"zentao_updated_at"`
	SyncedAt        *time.Time `json:"synced_at"`
	// 关联
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

func (Issue) TableName() string { return "issue" }

// IssueStatusLog 问题单状态变更日志
type IssueStatusLog struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	IssueID     uint64    `gorm:"index;not null" json:"issue_id"`
	Field       string    `gorm:"size:32;not null" json:"field"`
	OldValue    string    `gorm:"size:64;default:''" json:"old_value"`
	NewValue    string    `gorm:"size:64;not null" json:"new_value"`
	TriggerType string   `gorm:"size:16;default:'system'" json:"trigger_type"`
	OperatorID  *uint64   `json:"operator_id"`
	Remark      string    `gorm:"size:255;default:''" json:"remark"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (IssueStatusLog) TableName() string { return "issue_status_log" }
