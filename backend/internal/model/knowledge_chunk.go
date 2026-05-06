package model

import "time"

// KnowledgeChunk Chunk 文本与元数据，向量数据存储在 Milvus。
type KnowledgeChunk struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	DocID      uint64    `gorm:"index;not null" json:"doc_id"`
	ChunkIndex int       `gorm:"not null" json:"chunk_index"`
	ChunkText  string    `gorm:"type:text;not null" json:"chunk_text"`
	Metadata   JSON      `gorm:"type:json" json:"metadata"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KnowledgeChunk) TableName() string { return "knowledge_chunks" }
