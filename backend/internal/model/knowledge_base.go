package model

import "time"

// KnowledgeBase 项目知识库，按 project_id 强隔离。
type KnowledgeBase struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID    uint64    `gorm:"index;not null" json:"project_id"`
	Name         string    `gorm:"size:200;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description"`
	Status       int8      `gorm:"default:1" json:"status"`
	ChunkSize    int       `gorm:"default:500" json:"chunk_size"`
	ChunkOverlap int       `gorm:"default:50" json:"chunk_overlap"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KnowledgeBase) TableName() string { return "knowledge_bases" }
