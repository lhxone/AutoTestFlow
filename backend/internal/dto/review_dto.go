package dto

// ReviewListQuery Review列表查询
type ReviewListQuery struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID  uint64 `form:"project_id"`
	TaskID     uint64 `form:"task_id"`
	Status     string `form:"status"`
	ReviewerID uint64 `form:"reviewer_id"`
}

// ReviewActionRequest Review审核操作请求
type ReviewActionRequest struct {
	Action  string `json:"action" binding:"required,oneof=approve reject request_changes comment fail_regression reject_and_mark_error"`
	Comment string `json:"comment"`
}

// ReviewDetailResponse Review详情响应
type ReviewDetailResponse struct {
	ID          uint64           `json:"id"`
	Title       string           `json:"title"`
	Status      string           `json:"status"`
	IssueTitle  string           `json:"issue_title"`
	TestCases   []TestCaseVO     `json:"test_cases"`
	TestScripts []TestScriptVO   `json:"test_scripts"`
	TestDocs    []TestDocVO      `json:"test_docs"`
	Records     []ReviewRecordVO `json:"records"`
}

// TestCaseVO 测试用例视图
type TestCaseVO struct {
	ID             uint64 `json:"id"`
	Title          string `json:"title"`
	Category       string `json:"category"`
	Precondition   string `json:"precondition"`
	Steps          string `json:"steps"`
	Expected       string `json:"expected"`
	SelfTestResult string `json:"self_test_result"`
	Source         string `json:"source"`
}

// TestScriptVO 测试脚本视图
type TestScriptVO struct {
	ID          uint64 `json:"id"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
	Language    string `json:"language"`
	Source      string `json:"source"`
}

// TestDocVO 测试文档视图
type TestDocVO struct {
	ID      uint64 `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	DocType string `json:"doc_type"`
	Source  string `json:"source"`
}

// ReviewRecordVO Review记录视图
type ReviewRecordVO struct {
	ID           uint64 `json:"id"`
	ReviewerName string `json:"reviewer_name"`
	Action       string `json:"action"`
	Comment      string `json:"comment"`
	CreatedAt    string `json:"created_at"`
}
