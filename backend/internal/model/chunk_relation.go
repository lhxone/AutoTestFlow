package model

import "time"

// ChunkRelation 知识图谱中的 Chunk 关联边。
type ChunkRelation struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceChunkID uint64    `gorm:"uniqueIndex:uk_source_target_type;index;not null" json:"source_chunk_id"`
	TargetChunkID uint64    `gorm:"uniqueIndex:uk_source_target_type;index;not null" json:"target_chunk_id"`
	RelationType  string    `gorm:"size:30;uniqueIndex:uk_source_target_type;not null" json:"relation_type"`
	Score         float64   `gorm:"type:decimal(5,4)" json:"score"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ChunkRelation) TableName() string { return "chunk_relations" }
