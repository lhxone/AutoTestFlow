package model

import "time"

// TestTask 测试任务
type TestTask struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	IssueID      uint64     `gorm:"index;not null" json:"issue_id"`
	ProjectID    uint64     `gorm:"index;not null" json:"project_id"`
	AgentID      *uint64    `json:"agent_id"`
	SkillName    string     `gorm:"column:skill_name;size:64;default:'gen-test'" json:"workflow_name"`
	Status       string     `gorm:"size:32;index;default:'pending'" json:"status"`
	AIInput      JSON       `gorm:"type:json" json:"ai_input"`
	AIOutput     JSON       `gorm:"type:json" json:"ai_output"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	RetryCount   int        `gorm:"default:0" json:"retry_count"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedBy    *uint64    `json:"created_by"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	// 关联
	Issue   *Issue   `gorm:"foreignKey:IssueID" json:"issue,omitempty"`
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

func (TestTask) TableName() string { return "test_task" }

// 测试任务状态
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusWarning   = "warning"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

// TestCase 测试用例
type TestCase struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID         uint64    `gorm:"index;not null" json:"task_id"`
	IssueID        uint64    `gorm:"index;not null" json:"issue_id"`
	ProjectID      uint64    `gorm:"index;not null" json:"project_id"`
	Title          string    `gorm:"size:256;not null" json:"title"`
	Category       string    `gorm:"size:32;index;default:'normal'" json:"category"`
	Precondition   string    `gorm:"type:text" json:"precondition"`
	Steps          string    `gorm:"type:text" json:"steps"`
	Expected       string    `gorm:"type:text" json:"expected"`
	Actual         string    `gorm:"type:text" json:"actual"`
	SelfTestResult string    `gorm:"size:16;default:'pending'" json:"self_test_result"`
	Priority       int8      `gorm:"default:2" json:"priority"`
	CurrentVersion int       `gorm:"default:1" json:"current_version"`
	Source         string    `gorm:"size:16;default:'ai'" json:"source"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TestCase) TableName() string { return "test_case" }

// TestCaseVersion 测试用例版本历史
type TestCaseVersion struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TestCaseID   uint64    `gorm:"uniqueIndex:uk_case_version;not null" json:"test_case_id"`
	Version      int       `gorm:"uniqueIndex:uk_case_version;not null" json:"version"`
	Title        string    `gorm:"size:256;not null" json:"title"`
	Precondition string    `gorm:"type:text" json:"precondition"`
	Steps        string    `gorm:"type:text" json:"steps"`
	Expected     string    `gorm:"type:text" json:"expected"`
	Source       string    `gorm:"size:16;default:'ai'" json:"source"`
	ChangeNote   string    `gorm:"size:512;default:''" json:"change_note"`
	ChangedBy    *uint64   `gorm:"index" json:"changed_by"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TestCaseVersion) TableName() string { return "test_case_version" }

// TestScript 测试脚本
type TestScript struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID         uint64    `gorm:"index;not null" json:"task_id"`
	IssueID        uint64    `gorm:"index;not null" json:"issue_id"`
	ProjectID      uint64    `gorm:"index;not null" json:"project_id"`
	FilePath       string    `gorm:"size:512;not null" json:"file_path"`
	FileContent    string    `gorm:"type:longtext" json:"file_content"`
	Language       string    `gorm:"size:16;default:'typescript'" json:"language"`
	CurrentVersion int       `gorm:"default:1" json:"current_version"`
	Source         string    `gorm:"size:16;default:'ai'" json:"source"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TestScript) TableName() string { return "test_script" }

// TestScriptVersion 测试脚本版本历史
type TestScriptVersion struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TestScriptID uint64    `gorm:"uniqueIndex:uk_script_version;not null" json:"test_script_id"`
	Version      int       `gorm:"uniqueIndex:uk_script_version;not null" json:"version"`
	FileContent  string    `gorm:"type:longtext" json:"file_content"`
	Source       string    `gorm:"size:16;default:'ai'" json:"source"`
	ChangeNote   string    `gorm:"size:512;default:''" json:"change_note"`
	ChangedBy    *uint64   `gorm:"index" json:"changed_by"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TestScriptVersion) TableName() string { return "test_script_version" }

// TestDocument 测试文档
type TestDocument struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID         uint64    `gorm:"index;not null" json:"task_id"`
	IssueID        uint64    `gorm:"index;not null" json:"issue_id"`
	ProjectID      uint64    `gorm:"index;not null" json:"project_id"`
	Title          string    `gorm:"size:256;not null" json:"title"`
	FilePath       string    `gorm:"size:512;default:''" json:"file_path"`
	Content        string    `gorm:"type:longtext" json:"content"`
	DocType        string    `gorm:"size:32;default:'test_report'" json:"doc_type"`
	CurrentVersion int       `gorm:"default:1" json:"current_version"`
	Source         string    `gorm:"size:16;default:'ai'" json:"source"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TestDocument) TableName() string { return "test_document" }

type TaskEventLog struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    uint64    `gorm:"index;not null" json:"task_id"`
	Seq       int       `gorm:"not null;default:0" json:"seq"`
	Type      string    `gorm:"size:32;not null;default:''" json:"type"`
	Stage     string    `gorm:"size:64;not null;default:''" json:"stage"`
	Status    string    `gorm:"size:32;not null;default:''" json:"status"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	Data      JSON      `gorm:"type:json" json:"data"`
	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
}

func (TaskEventLog) TableName() string { return "task_event_log" }
