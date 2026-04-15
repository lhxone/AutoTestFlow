package dto

// CreateTestTaskRequest 创建测试任务请求(手动触发)
type CreateTestTaskRequest struct {
	IssueID      uint64  `json:"issue_id" binding:"required"`
	AgentID      *uint64 `json:"agent_id"`
	WorkflowName string  `json:"workflow_name"`
}

// TestTaskListQuery 测试任务列表查询
type TestTaskListQuery struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID uint64 `form:"project_id"`
	IssueID   uint64 `form:"issue_id"`
	Keyword   string `form:"keyword"`
	Status    string `form:"status"`
}

// TriggerExecutionRequest 触发测试执行请求
type TriggerExecutionRequest struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	Branch    string `json:"branch"`
}

// UpdateTestCaseRequest 人工修改测试用例
type UpdateTestCaseRequest struct {
	Title        string `json:"title"`
	Precondition string `json:"precondition"`
	Steps        string `json:"steps"`
	Expected     string `json:"expected"`
	ChangeNote   string `json:"change_note" binding:"required"`
}

// UpdateTestScriptRequest 人工修改测试脚本
type UpdateTestScriptRequest struct {
	FileContent string `json:"file_content" binding:"required"`
	ChangeNote  string `json:"change_note" binding:"required"`
}

// TestCaseListQuery 测试用例列表查询
type TestCaseListQuery struct {
	Page           int    `form:"page" binding:"omitempty,min=1"`
	PageSize       int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID      uint64 `form:"project_id"`
	IssueID        uint64 `form:"issue_id"`
	TaskID         uint64 `form:"task_id"`
	Keyword        string `form:"keyword"`
	Category       string `form:"category"`
	Source         string `form:"source"`
	SelfTestResult string `form:"self_test_result"`
}

// ExecutionListQuery 执行记录查询
type ExecutionListQuery struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID uint64 `form:"project_id"`
	TaskID    uint64 `form:"task_id"`
	Status    string `form:"status"`
}
