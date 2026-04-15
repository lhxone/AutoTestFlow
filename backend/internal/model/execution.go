package model

import "time"

// TestExecution 测试执行记录
type TestExecution struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID    uint64     `gorm:"index;not null" json:"project_id"`
	TriggerType  string     `gorm:"size:16;index;default:'schedule'" json:"trigger_type"`
	TriggerBy    *uint64    `json:"trigger_by"`
	Branch       string     `gorm:"size:128;default:''" json:"branch"`
	CommitHash   string     `gorm:"size:64;default:''" json:"commit_hash"`
	CIJobID      string     `gorm:"size:128;default:''" json:"ci_job_id"`
	CIJobURL     string     `gorm:"size:512;default:''" json:"ci_job_url"`
	Status       string     `gorm:"size:32;index;default:'pending'" json:"status"`
	TotalCases   int        `gorm:"default:0" json:"total_cases"`
	PassedCases  int        `gorm:"default:0" json:"passed_cases"`
	FailedCases  int        `gorm:"default:0" json:"failed_cases"`
	SkippedCases int        `gorm:"default:0" json:"skipped_cases"`
	PassRate     float64    `gorm:"type:decimal(5,2);default:0" json:"pass_rate"`
	DurationSec  int        `gorm:"default:0" json:"duration_sec"`
	ResultDetail JSON       `gorm:"type:json" json:"result_detail"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TestExecution) TableName() string { return "test_execution" }

// TestExecutionIssue 执行-问题单关联
type TestExecutionIssue struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ExecutionID uint64    `gorm:"index;not null" json:"execution_id"`
	IssueID     uint64    `gorm:"index;not null" json:"issue_id"`
	TestTaskID  *uint64   `json:"test_task_id"`
	CaseTotal   int       `gorm:"default:0" json:"case_total"`
	CasePassed  int       `gorm:"default:0" json:"case_passed"`
	CaseFailed  int       `gorm:"default:0" json:"case_failed"`
	Result      string    `gorm:"size:16;default:'pending'" json:"result"`
	FailDetail  JSON      `gorm:"type:json" json:"fail_detail"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TestExecutionIssue) TableName() string { return "test_execution_issue" }

// ManualIntervention 人工介入记录
type ManualIntervention struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ExecutionID      *uint64   `gorm:"index" json:"execution_id"`
	TestTaskID       *uint64   `json:"test_task_id"`
	IssueID          uint64    `gorm:"index;not null" json:"issue_id"`
	ProjectID        uint64    `gorm:"not null" json:"project_id"`
	OperatorID       uint64    `gorm:"index;not null" json:"operator_id"`
	InterventionType string    `gorm:"size:32;not null" json:"intervention_type"`
	Description      string    `gorm:"type:text" json:"description"`
	BeforeSnapshot   string    `gorm:"type:longtext" json:"before_snapshot"`
	AfterSnapshot    string    `gorm:"type:longtext" json:"after_snapshot"`
	Status           string    `gorm:"size:16;default:'completed'" json:"status"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	// 关联
	Operator *User `gorm:"foreignKey:OperatorID" json:"operator,omitempty"`
}

func (ManualIntervention) TableName() string { return "manual_intervention" }

// TestReport 测试报告
type TestReport struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ExecutionID     uint64    `gorm:"index;not null" json:"execution_id"`
	ProjectID       uint64    `gorm:"index;not null" json:"project_id"`
	Title           string    `gorm:"size:256;not null" json:"title"`
	Summary         string    `gorm:"type:text" json:"summary"`
	Content         string    `gorm:"type:longtext" json:"content"`
	TotalIssues     int       `gorm:"default:0" json:"total_issues"`
	TotalCases      int       `gorm:"default:0" json:"total_cases"`
	PassedCases     int       `gorm:"default:0" json:"passed_cases"`
	FailedCases     int       `gorm:"default:0" json:"failed_cases"`
	PassRate        float64   `gorm:"type:decimal(5,2);default:0" json:"pass_rate"`
	HasIntervention int8      `gorm:"default:0" json:"has_intervention"`
	LastModifier    string    `gorm:"size:64;default:''" json:"last_modifier"`
	ReportURL       string    `gorm:"size:512;default:''" json:"report_url"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TestReport) TableName() string { return "test_report" }

// NotificationLog 通知发送日志
type NotificationLog struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ReportID     *uint64    `gorm:"index" json:"report_id"`
	Channel      string     `gorm:"size:16;default:'email'" json:"channel"`
	Recipient    string     `gorm:"size:128;not null" json:"recipient"`
	Subject      string     `gorm:"size:256;default:''" json:"subject"`
	Content      string     `gorm:"type:text" json:"content"`
	Status       string     `gorm:"size:16;index;default:'pending'" json:"status"`
	ErrorMessage string     `gorm:"size:512;default:''" json:"error_message"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (NotificationLog) TableName() string { return "notification_log" }
