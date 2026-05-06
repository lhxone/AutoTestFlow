package model

import "time"

const (
	KnowledgeDocumentStatusPending = "pending"
	KnowledgeDocumentStatusParsing = "parsing"
	KnowledgeDocumentStatusIndexed = "indexed"
	KnowledgeDocumentStatusFailed  = "failed"
)

// KnowledgeDocument 知识库文档原文。
type KnowledgeDocument struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	KBID        uint64    `gorm:"column:kb_id;index;not null" json:"kb_id"`
	SourceType  string    `gorm:"size:50;not null" json:"source_type"`
	SourcePath  string    `gorm:"size:500" json:"source_path"`
	Title       string    `gorm:"size:300" json:"title"`
	Content     string    `gorm:"type:longtext" json:"content"`
	ContentSize int       `gorm:"default:0" json:"content_size"`
	ChunkCount  int       `gorm:"default:0" json:"chunk_count"`
	Status      string    `gorm:"size:20;default:'pending';index" json:"status"`
	ErrorMsg    string    `gorm:"type:text" json:"error_msg"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KnowledgeDocument) TableName() string { return "knowledge_documents" }
