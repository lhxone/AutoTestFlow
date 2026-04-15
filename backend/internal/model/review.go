package model

import "time"

// Review 状态
const (
	ReviewStatusPending          = "pending"
	ReviewStatusApproved         = "approved"
	ReviewStatusRejected         = "rejected"
	ReviewStatusChangesRequested = "changes_requested"
)

// ReviewTask Review审核任务
type ReviewTask struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TestTaskID  uint64     `gorm:"index;not null" json:"test_task_id"`
	IssueID     uint64     `gorm:"index;not null" json:"issue_id"`
	ProjectID   uint64     `gorm:"index;not null" json:"project_id"`
	Title       string     `gorm:"size:256;not null" json:"title"`
	Status      string     `gorm:"size:32;index;default:'pending'" json:"status"`
	ReviewerID  *uint64    `gorm:"index" json:"reviewer_id"`
	SubmittedBy *uint64    `json:"submitted_by"`
	ReviewNote  string     `gorm:"type:text" json:"review_note"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	// 关联
	TestTask *TestTask `gorm:"foreignKey:TestTaskID" json:"test_task,omitempty"`
	Issue    *Issue    `gorm:"foreignKey:IssueID" json:"issue,omitempty"`
	Reviewer *User     `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}

func (ReviewTask) TableName() string { return "review_task" }

// ReviewRecord Review审核记录
type ReviewRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ReviewTaskID uint64    `gorm:"index;not null" json:"review_task_id"`
	ReviewerID   uint64    `gorm:"index;not null" json:"reviewer_id"`
	Action       string    `gorm:"size:32;not null" json:"action"` // approve/reject/request_changes/comment
	Comment      string    `gorm:"type:text" json:"comment"`
	DiffSnapshot string    `gorm:"type:longtext" json:"diff_snapshot"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	// 关联
	Reviewer *User `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}

func (ReviewRecord) TableName() string { return "review_record" }
