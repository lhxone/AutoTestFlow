package dto

// IssueListQuery 问题单列表查询
type IssueListQuery struct {
	Page          int    `form:"page" binding:"omitempty,min=1"`
	PageSize      int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID     uint64 `form:"project_id"`
	ProjectSetID  uint64 `form:"project_set_id"`
	ZentaoStatus  string `form:"zentao_status"`
	TestStatus    string `form:"test_status"`
	Branch        string `form:"branch"`
	Keyword       string `form:"keyword"`
	Assignee      string `form:"assignee"`
}

// SyncIssuesRequest 同步问题单请求
type SyncIssuesRequest struct {
	ProjectID uint64 `json:"project_id" binding:"required"`
	FullSync  bool   `json:"full_sync"` // true=全量 false=增量
}

// UpdateTestStatusRequest 更新测试状态请求
type UpdateTestStatusRequest struct {
	TestStatus string `json:"test_status" binding:"required"`
	Remark     string `json:"remark"`
}
