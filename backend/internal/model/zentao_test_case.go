package model

import "time"

// 用例同步状态枚举
const (
	ZentaoTestCaseStatusSynced    = "synced"
	ZentaoTestCaseStatusGenerating = "generating"
	ZentaoTestCaseStatusGenerated  = "generated"
	ZentaoTestCaseStatusFailed    = "failed"
)

// ZentaoTestCase 禅道用例（独立模型，不复用 Issue/Bug）
type ZentaoTestCase struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ZentaoID  int       `gorm:"uniqueIndex:uk_ztc_zentao_project;not null" json:"zentao_id"`
	ProductID uint64    `gorm:"index;not null" json:"product_id"`
	Title     string    `gorm:"size:512;not null" json:"title"`
	Precondition string `gorm:"type:text" json:"precondition"`
	Keywords  string    `gorm:"size:255" json:"keywords"`
	Priority  int8      `gorm:"default:3" json:"priority"`
	Type      string    `gorm:"size:32" json:"type"` // feature/performance/config/install/security/interface/unit/other
	Stage     string    `gorm:"size:32" json:"stage"` // unittest/feature/intergrate/system/smoke/bvt
	Status    string    `gorm:"size:32;default:'normal'" json:"status"` // wait/normal/blocked/investigate
	TestStatus string   `gorm:"size:32;default:'pending'" json:"test_status"` // pending/generating/generated/failed
	Branch    string    `gorm:"size:128" json:"branch"`
	Module    string    `gorm:"size:255" json:"module"`
	Steps     string    `gorm:"type:text" json:"steps"` // 合并后的步骤文本
	Expected  string    `gorm:"type:text" json:"expected"` // 合并后的预期结果
	OpenedBy  string    `gorm:"size:64" json:"opened_by"`
	CreatedBy string    `gorm:"size:64" json:"created_by"`
	SyncedAt  *time.Time `json:"synced_at"`
	ZentaoUpdatedAt *time.Time `json:"zentao_updated_at"`
}

func (ZentaoTestCase) TableName() string { return "zentao_test_case" }
