package repository

import (
	"auto-test-flow/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgeRepo struct {
	db *gorm.DB
}

type KnowledgeStats struct {
	DocumentCount int64 `json:"document_count"`
	ChunkCount    int64 `json:"chunk_count"`
	VectorCount   int64 `json:"vector_count"`
	GraphNodes    int64 `json:"graph_nodes"`
	GraphEdges    int64 `json:"graph_edges"`
}

type KnowledgeGraphRow struct {
	DocID        uint64   `json:"doc_id"`
	DocTitle     string   `json:"doc_title"`
	ChunkID      uint64   `json:"chunk_id"`
	ChunkIndex   int      `json:"chunk_index"`
	ChunkText    string   `json:"chunk_text"`
	TargetID     *uint64  `json:"target_id"`
	RelationType *string  `json:"relation_type"`
	Score        *float64 `json:"score"`
}

func NewKnowledgeRepo() *KnowledgeRepo {
	return &KnowledgeRepo{db: DB}
}

func (r *KnowledgeRepo) CreateKB(kb *model.KnowledgeBase) error {
	return r.db.Create(kb).Error
}

func (r *KnowledgeRepo) ListKB(projectID uint64, keyword string, offset, limit int) ([]model.KnowledgeBase, int64, error) {
	query := r.db.Model(&model.KnowledgeBase{}).Where("project_id = ?", projectID)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.KnowledgeBase
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *KnowledgeRepo) GetKB(projectID, id uint64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.Where("project_id = ? AND id = ?", projectID, id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *KnowledgeRepo) GetKBByID(id uint64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.First(&kb, id).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *KnowledgeRepo) UpdateKB(kb *model.KnowledgeBase) error {
	return r.db.Model(&model.KnowledgeBase{}).
		Where("id = ? AND project_id = ?", kb.ID, kb.ProjectID).
		Updates(map[string]any{
			"name":          kb.Name,
			"description":   kb.Description,
			"status":        kb.Status,
			"chunk_size":    kb.ChunkSize,
			"chunk_overlap": kb.ChunkOverlap,
		}).Error
}

func (r *KnowledgeRepo) DeleteKB(projectID, id uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var chunkIDs []uint64
		if err := tx.Model(&model.KnowledgeChunk{}).
			Where("doc_id IN (?)", tx.Model(&model.KnowledgeDocument{}).Select("id").Where("kb_id = ?", id)).
			Pluck("id", &chunkIDs).Error; err != nil {
			return err
		}
		if len(chunkIDs) > 0 {
			if err := tx.Where("source_chunk_id IN ? OR target_chunk_id IN ?", chunkIDs, chunkIDs).Delete(&model.ChunkRelation{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("doc_id IN (?)", tx.Model(&model.KnowledgeDocument{}).Select("id").Where("kb_id = ?", id)).
			Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", id).Delete(&model.KnowledgeDocument{}).Error; err != nil {
			return err
		}
		return tx.Where("project_id = ? AND id = ?", projectID, id).Delete(&model.KnowledgeBase{}).Error
	})
}

func (r *KnowledgeRepo) CreateDocument(doc *model.KnowledgeDocument) error {
	return r.db.Create(doc).Error
}

func (r *KnowledgeRepo) ListDocuments(kbID uint64, offset, limit int) ([]model.KnowledgeDocument, int64, error) {
	query := r.db.Model(&model.KnowledgeDocument{}).Where("kb_id = ?", kbID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var docs []model.KnowledgeDocument
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&docs).Error
	return docs, total, err
}

func (r *KnowledgeRepo) GetDocument(kbID, docID uint64) (*model.KnowledgeDocument, error) {
	var doc model.KnowledgeDocument
	err := r.db.Where("kb_id = ? AND id = ?", kbID, docID).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *KnowledgeRepo) UpdateDocumentStatus(docID uint64, status, errMsg string, chunkCount int) error {
	return r.db.Model(&model.KnowledgeDocument{}).Where("id = ?", docID).Updates(map[string]any{
		"status":      status,
		"error_msg":   errMsg,
		"chunk_count": chunkCount,
	}).Error
}

func (r *KnowledgeRepo) DeleteDocument(kbID, docID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ids []uint64
		if err := tx.Model(&model.KnowledgeChunk{}).Where("doc_id = ?", docID).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Where("source_chunk_id IN ? OR target_chunk_id IN ?", ids, ids).Delete(&model.ChunkRelation{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("doc_id = ?", docID).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		return tx.Where("kb_id = ? AND id = ?", kbID, docID).Delete(&model.KnowledgeDocument{}).Error
	})
}

func (r *KnowledgeRepo) ReplaceChunks(docID uint64, chunks []model.KnowledgeChunk) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ids []uint64
		if err := tx.Model(&model.KnowledgeChunk{}).Where("doc_id = ?", docID).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Where("source_chunk_id IN ? OR target_chunk_id IN ?", ids, ids).Delete(&model.ChunkRelation{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("doc_id = ?", docID).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) > 0 {
			if err := tx.Create(&chunks).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.KnowledgeDocument{}).Where("id = ?", docID).Update("chunk_count", len(chunks)).Error
	})
}

func (r *KnowledgeRepo) ListChunksByKB(kbID uint64) ([]model.KnowledgeChunk, error) {
	var chunks []model.KnowledgeChunk
	err := r.db.Joins("JOIN knowledge_documents d ON d.id = knowledge_chunks.doc_id").
		Where("d.kb_id = ?", kbID).
		Order("knowledge_chunks.doc_id ASC, knowledge_chunks.chunk_index ASC").
		Find(&chunks).Error
	return chunks, err
}

func (r *KnowledgeRepo) ListActiveKBs(projectID uint64) ([]model.KnowledgeBase, error) {
	var list []model.KnowledgeBase
	err := r.db.Where("project_id = ? AND status = 1", projectID).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *KnowledgeRepo) UpsertRelations(relations []model.ChunkRelation) error {
	if len(relations) == 0 {
		return nil
	}
	const batchSize = 1000
	for i := 0; i < len(relations); i += batchSize {
		end := i + batchSize
		if end > len(relations) {
			end = len(relations)
		}
		batch := relations[i:end]
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_chunk_id"}, {Name: "target_chunk_id"}, {Name: "relation_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"score"}),
		}).Create(&batch).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *KnowledgeRepo) ClearRelationsByKB(kbID uint64) error {
	return r.db.Exec(`
		DELETE cr FROM chunk_relations cr
		JOIN knowledge_chunks c ON c.id = cr.source_chunk_id OR c.id = cr.target_chunk_id
		JOIN knowledge_documents d ON d.id = c.doc_id
		WHERE d.kb_id = ?`, kbID).Error
}

func (r *KnowledgeRepo) Stats(kbID uint64) (*KnowledgeStats, error) {
	stats := &KnowledgeStats{}
	if err := r.db.Model(&model.KnowledgeDocument{}).Where("kb_id = ?", kbID).Count(&stats.DocumentCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Table("knowledge_chunks c").Joins("JOIN knowledge_documents d ON d.id = c.doc_id").
		Where("d.kb_id = ?", kbID).Count(&stats.ChunkCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Table("chunk_relations cr").Joins("JOIN knowledge_chunks c ON c.id = cr.source_chunk_id").
		Joins("JOIN knowledge_documents d ON d.id = c.doc_id").Where("d.kb_id = ?", kbID).Count(&stats.GraphEdges).Error; err != nil {
		return nil, err
	}
	stats.VectorCount = stats.ChunkCount
	stats.GraphNodes = stats.DocumentCount + stats.ChunkCount
	return stats, nil
}

func (r *KnowledgeRepo) GraphRows(kbID uint64) ([]KnowledgeGraphRow, error) {
	var rows []KnowledgeGraphRow
	err := r.db.Raw(`
		SELECT d.id AS doc_id, d.title AS doc_title, c.id AS chunk_id, c.chunk_index, c.chunk_text,
		       cr.target_chunk_id AS target_id, cr.relation_type, cr.score
		FROM knowledge_documents d
		JOIN knowledge_chunks c ON c.doc_id = d.id
		LEFT JOIN chunk_relations cr ON cr.source_chunk_id = c.id
		WHERE d.kb_id = ?
		ORDER BY d.id ASC, c.chunk_index ASC`, kbID).Scan(&rows).Error
	return rows, err
}
