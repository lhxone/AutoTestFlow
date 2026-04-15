package model

// Project 项目
type Project struct {
	BaseModel
	Name              string `gorm:"size:128;not null" json:"name"`
	Description       string `gorm:"type:text" json:"description"`
	FuncDocPath       string `gorm:"size:512;default:''" json:"func_doc_path"`
	DesignDocPath     string `gorm:"size:512;default:''" json:"design_doc_path"`
	DBDocPath         string `gorm:"size:512;default:''" json:"db_doc_path"`
	TestDocPath       string `gorm:"size:512;default:''" json:"test_doc_path"`
	ExtraFilesPath    string `gorm:"size:512;default:''" json:"extra_files_path"`
	GitRepoURL        string `gorm:"size:512;default:''" json:"git_repo_url"`
	GitBranch         string `gorm:"size:128;default:'main'" json:"git_branch"`
	ZentaoProjectID   *int   `gorm:"index" json:"zentao_project_id"`
	ZentaoProjectName string `gorm:"size:128;default:''" json:"zentao_project_name"`
	ZentaoBranch      string `gorm:"size:128;default:''" json:"zentao_branch"`
	Status            int8   `gorm:"default:1;not null" json:"status"`
	OwnerID           *uint64 `gorm:"index" json:"owner_id"`
	// 关联
	Owner   *User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Members []ProjectMember `gorm:"foreignKey:ProjectID" json:"members,omitempty"`
}

func (Project) TableName() string { return "project" }

// ProjectMember 项目成员
type ProjectMember struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID uint64 `gorm:"uniqueIndex:uk_project_user;not null" json:"project_id"`
	UserID    uint64 `gorm:"uniqueIndex:uk_project_user;index;not null" json:"user_id"`
	Role      string `gorm:"size:32;default:'member'" json:"role"`
	// 关联
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ProjectMember) TableName() string { return "project_member" }
