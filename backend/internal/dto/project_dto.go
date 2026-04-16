package dto

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name              string  `json:"name" binding:"required,max=128"`
	Description       string  `json:"description"`
	FuncDocPath       string  `json:"func_doc_path"`
	DesignDocPath     string  `json:"design_doc_path"`
	DBDocPath         string  `json:"db_doc_path"`
	TestDocPath       string  `json:"test_doc_path"`
	ExtraFilesPath    string  `json:"extra_files_path"`
	GitRepoURL        string  `json:"git_repo_url"`
	GitBranch         string  `json:"git_branch"`
	GitPullInterval   int     `json:"git_pull_interval"`
	ZentaoProjectID   *int    `json:"zentao_project_id"`
	ZentaoProjectName string  `json:"zentao_project_name"`
	ZentaoBranch      string  `json:"zentao_branch"`
	OwnerID           *uint64 `json:"owner_id"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name              string  `json:"name" binding:"max=128"`
	Description       string  `json:"description"`
	FuncDocPath       string  `json:"func_doc_path"`
	DesignDocPath     string  `json:"design_doc_path"`
	DBDocPath         string  `json:"db_doc_path"`
	TestDocPath       string  `json:"test_doc_path"`
	ExtraFilesPath    string  `json:"extra_files_path"`
	GitRepoURL        string  `json:"git_repo_url"`
	GitBranch         string  `json:"git_branch"`
	GitPullInterval   *int    `json:"git_pull_interval"`
	ZentaoProjectID   *int    `json:"zentao_project_id"`
	ZentaoProjectName string  `json:"zentao_project_name"`
	ZentaoBranch      string  `json:"zentao_branch"`
	Status            *int8   `json:"status"`
	OwnerID           *uint64 `json:"owner_id"`
}

// ProjectListQuery 项目列表查询
type ProjectListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   *int8  `form:"status"`
}

// ProjectIssueSyncLogQuery 项目问题单同步日志查询
type ProjectIssueSyncLogQuery struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// IssueSyncLogListQuery 全局采集记录查询
type IssueSyncLogListQuery struct {
	Page      int     `form:"page" binding:"omitempty,min=1"`
	PageSize  int     `form:"page_size" binding:"omitempty,min=1,max=100"`
	ProjectID *uint64 `form:"project_id"`
}

// IssueSyncLogDetailQuery 采集详情查询
type IssueSyncLogDetailQuery struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// ProjectMemberRequest 项目成员请求
type ProjectMemberRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=owner test_lead tester dev_lead member"`
}
