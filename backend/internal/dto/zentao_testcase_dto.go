package dto

type ZentaoTestCaseListQuery struct {
	ProjectID    uint64 `form:"project_id"`
	ProductID    uint64 `form:"product_id"`
	TestStatus   string `form:"test_status"`
	Branch       string `form:"branch"`
	Type         string `form:"type"`
	Keyword      string `form:"keyword"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

type SyncTestCasesRequest struct {
	ProjectID uint64 `json:"project_id"`
	FullSync  bool   `json:"full_sync"`
}

type GenerateTestScriptRequest struct {
	TestCaseID uint64 `json:"test_case_id"`
	AgentID    *uint64 `json:"agent_id"`
}
