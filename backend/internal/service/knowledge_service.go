package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type KnowledgeService struct {
	repo          *repository.KnowledgeRepo
	configService *KnowledgeBaseConfigService
	pipeline      *RAGPipeline
	vectorStore   VectorStore
	logger        *zap.Logger
}

type CreateKnowledgeBaseRequest struct {
	ProjectID    uint64 `json:"project_id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Status       int8   `json:"status"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

type UpdateKnowledgeBaseRequest struct {
	ProjectID    uint64 `json:"project_id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Status       int8   `json:"status"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

type KnowledgeDocumentRequest struct {
	ProjectID  uint64 `json:"project_id" form:"project_id" binding:"required"`
	SourceType string `json:"source_type" form:"source_type"`
	SourcePath string `json:"source_path" form:"source_path"`
	Title      string `json:"title" form:"title"`
	Content    string `json:"content" form:"content"`
}

type BatchKnowledgeDocumentRequest struct {
	ProjectID uint64                     `json:"project_id" binding:"required"`
	Documents []KnowledgeDocumentRequest `json:"documents" binding:"required,min=1"`
}

type KnowledgeQueryRequest struct {
	ProjectID uint64   `json:"project_id" binding:"required"`
	Query     string   `json:"query" binding:"required"`
	TopK      int      `json:"top_k"`
	Keywords  []string `json:"keywords"`
}

type KnowledgeGraph struct {
	Nodes []KnowledgeGraphNode `json:"nodes"`
	Edges []KnowledgeGraphEdge `json:"edges"`
}

type KnowledgeGraphNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Category string         `json:"category"`
	Value    int            `json:"value"`
	Meta     map[string]any `json:"meta"`
}

type KnowledgeGraphEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Type   string  `json:"type"`
	Score  float64 `json:"score"`
}

func NewKnowledgeService(logger *zap.Logger) *KnowledgeService {
	cfgSvc := DefaultKnowledgeConfig
	if cfgSvc == nil {
		cfgSvc = InitKnowledgeBaseConfig(logger)
	}
	var store VectorStore
	cfg := cfgSvc.Current()
	if cfg.Enabled {
		initCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if created, err := NewVectorStore(initCtx, cfg); err != nil {
			if logger != nil {
				logger.Warn("初始化知识库向量存储失败", zap.Error(err))
			}
		} else {
			store = created
		}
	}
	return &KnowledgeService{
		repo:          repository.NewKnowledgeRepo(),
		configService: cfgSvc,
		vectorStore:   store,
		pipeline:      NewRAGPipeline(cfgSvc, store),
		logger:        logger,
	}
}

func (s *KnowledgeService) GetConfig() KnowledgeBaseConfig {
	return s.configService.Current()
}

func (s *KnowledgeService) SaveConfig(ctx context.Context, cfg KnowledgeBaseConfig, operatorID uint64) error {
	oldStore := s.vectorStore
	newCfg := normalizeKnowledgeBaseConfig(cfg)
	var newStore VectorStore
	if newCfg.Enabled {
		initCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		store, err := NewVectorStore(initCtx, newCfg)
		if err != nil {
			return err
		}
		newStore = store
	}
	if err := s.configService.Save(newCfg, operatorID); err != nil {
		if newStore != nil {
			_ = newStore.Close(ctx)
		}
		return err
	}
	s.vectorStore = newStore
	s.pipeline = NewRAGPipeline(s.configService, newStore)
	if oldStore != nil {
		_ = oldStore.Close(ctx)
	}
	return nil
}

func (s *KnowledgeService) CreateKB(req CreateKnowledgeBaseRequest) (*model.KnowledgeBase, error) {
	cfg := s.configService.Current()
	kb := &model.KnowledgeBase{
		ProjectID:    req.ProjectID,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Status:       req.Status,
		ChunkSize:    req.ChunkSize,
		ChunkOverlap: req.ChunkOverlap,
	}
	if kb.Status == 0 {
		kb.Status = 1
	}
	if kb.ChunkSize <= 0 {
		kb.ChunkSize = cfg.ChunkSize
	}
	if kb.ChunkOverlap < 0 || kb.ChunkOverlap >= kb.ChunkSize {
		kb.ChunkOverlap = cfg.ChunkOverlap
	}
	return kb, s.repo.CreateKB(kb)
}

func (s *KnowledgeService) ListKB(projectID uint64, keyword string, offset, limit int) ([]model.KnowledgeBase, int64, error) {
	return s.repo.ListKB(projectID, strings.TrimSpace(keyword), offset, limit)
}

func (s *KnowledgeService) GetKB(projectID, id uint64) (*model.KnowledgeBase, error) {
	return s.repo.GetKB(projectID, id)
}

func (s *KnowledgeService) UpdateKB(id uint64, req UpdateKnowledgeBaseRequest) (*model.KnowledgeBase, error) {
	kb, err := s.repo.GetKB(req.ProjectID, id)
	if err != nil {
		return nil, err
	}
	kb.Name = strings.TrimSpace(req.Name)
	kb.Description = strings.TrimSpace(req.Description)
	kb.Status = req.Status
	kb.ChunkSize = req.ChunkSize
	kb.ChunkOverlap = req.ChunkOverlap
	if kb.Status != 0 {
		kb.Status = 1
	}
	if kb.ChunkSize <= 0 {
		kb.ChunkSize = s.configService.Current().ChunkSize
	}
	if kb.ChunkOverlap < 0 || kb.ChunkOverlap >= kb.ChunkSize {
		kb.ChunkOverlap = s.configService.Current().ChunkOverlap
	}
	return kb, s.repo.UpdateKB(kb)
}

func (s *KnowledgeService) DeleteKB(ctx context.Context, projectID, id uint64) error {
	if _, err := s.repo.GetKB(projectID, id); err != nil {
		return err
	}
	if s.vectorStore != nil {
		if err := s.vectorStore.DeleteKnowledgeBase(ctx, projectID, id); err != nil && s.logger != nil {
			s.logger.Warn("删除知识库向量分区失败", zap.Uint64("project_id", projectID), zap.Uint64("kb_id", id), zap.Error(err))
		}
	}
	return s.repo.DeleteKB(projectID, id)
}

func (s *KnowledgeService) AddDocument(ctx context.Context, kbID uint64, req KnowledgeDocumentRequest) (*model.KnowledgeDocument, error) {
	kb, err := s.repo.GetKB(req.ProjectID, kbID)
	if err != nil {
		return nil, err
	}
	content, err := s.pipeline.LoadSource(ctx, req.SourceType, req.SourcePath, req.Content)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(req.SourcePath)
	}
	if title == "" {
		title = "manual-" + shortHash(content)
	}
	doc := &model.KnowledgeDocument{
		KBID:        kbID,
		SourceType:  firstDocString(req.SourceType, "manual"),
		SourcePath:  strings.TrimSpace(req.SourcePath),
		Title:       title,
		Content:     content,
		ContentSize: len([]rune(content)),
		Status:      model.KnowledgeDocumentStatusPending,
	}
	if err := s.repo.CreateDocument(doc); err != nil {
		return nil, err
	}
	if err := s.RebuildDocument(ctx, req.ProjectID, kb.ID, doc.ID); err != nil {
		return doc, err
	}
	return s.repo.GetDocument(kbID, doc.ID)
}

func (s *KnowledgeService) BatchAddDocuments(ctx context.Context, kbID uint64, req BatchKnowledgeDocumentRequest) ([]model.KnowledgeDocument, error) {
	docs := make([]model.KnowledgeDocument, 0, len(req.Documents))
	for _, item := range req.Documents {
		item.ProjectID = req.ProjectID
		doc, err := s.AddDocument(ctx, kbID, item)
		if err != nil {
			return docs, err
		}
		docs = append(docs, *doc)
	}
	return docs, nil
}

func (s *KnowledgeService) ListDocuments(projectID, kbID uint64, offset, limit int) ([]model.KnowledgeDocument, int64, error) {
	if _, err := s.repo.GetKB(projectID, kbID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListDocuments(kbID, offset, limit)
}

func (s *KnowledgeService) RebuildDocument(ctx context.Context, projectID, kbID, docID uint64) error {
	kb, err := s.repo.GetKB(projectID, kbID)
	if err != nil {
		return err
	}
	doc, err := s.repo.GetDocument(kbID, docID)
	if err != nil {
		return err
	}
	_ = s.repo.UpdateDocumentStatus(docID, model.KnowledgeDocumentStatusParsing, "", 0)
	chunks, err := s.pipeline.Split(ctx, doc, kb)
	if err != nil {
		_ = s.repo.UpdateDocumentStatus(docID, model.KnowledgeDocumentStatusFailed, err.Error(), 0)
		return err
	}
	if err := s.repo.ReplaceChunks(docID, chunks); err != nil {
		_ = s.repo.UpdateDocumentStatus(docID, model.KnowledgeDocumentStatusFailed, err.Error(), 0)
		return err
	}
	storedChunks, err := s.repo.ListChunksByKB(kbID)
	if err != nil {
		return err
	}
	docChunks := make([]model.KnowledgeChunk, 0, len(chunks))
	for _, chunk := range storedChunks {
		if chunk.DocID == docID {
			docChunks = append(docChunks, chunk)
		}
	}
	if s.configService.Current().Enabled && s.vectorStore != nil {
		ids := make([]string, 0, len(docChunks))
		for _, chunk := range docChunks {
			ids = append(ids, vectorIDForChunk(chunk.ID))
		}
		_ = s.vectorStore.DeleteByIDs(ctx, projectID, kbID, ids)
		if _, err := s.pipeline.StoreChunks(ctx, projectID, kbID, docChunks); err != nil {
			_ = s.repo.UpdateDocumentStatus(docID, model.KnowledgeDocumentStatusFailed, err.Error(), len(docChunks))
			return err
		}
	}
	if err := s.rebuildRelations(kbID); err != nil && s.logger != nil {
		s.logger.Warn("重建 chunk 关联失败", zap.Uint64("kb_id", kbID), zap.Error(err))
	}
	return s.repo.UpdateDocumentStatus(docID, model.KnowledgeDocumentStatusIndexed, "", len(docChunks))
}

func (s *KnowledgeService) DeleteDocument(ctx context.Context, projectID, kbID, docID uint64) error {
	if _, err := s.repo.GetKB(projectID, kbID); err != nil {
		return err
	}
	chunks, _ := s.repo.ListChunksByKB(kbID)
	ids := make([]string, 0)
	for _, chunk := range chunks {
		if chunk.DocID == docID {
			ids = append(ids, vectorIDForChunk(chunk.ID))
		}
	}
	if s.vectorStore != nil {
		_ = s.vectorStore.DeleteByIDs(ctx, projectID, kbID, ids)
	}
	return s.repo.DeleteDocument(kbID, docID)
}

func (s *KnowledgeService) RebuildKB(ctx context.Context, projectID, kbID uint64) error {
	if _, err := s.repo.GetKB(projectID, kbID); err != nil {
		return err
	}
	docs, _, err := s.repo.ListDocuments(kbID, 0, 100000)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if err := s.RebuildDocument(ctx, projectID, kbID, doc.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *KnowledgeService) Query(ctx context.Context, kbID uint64, req KnowledgeQueryRequest) ([]VectorSearchResult, error) {
	if _, err := s.repo.GetKB(req.ProjectID, kbID); err != nil {
		return nil, err
	}
	return s.pipeline.Retrieve(ctx, req.ProjectID, kbID, strings.TrimSpace(req.Query), req.TopK, req.Keywords)
}

func (s *KnowledgeService) Stats(projectID, kbID uint64) (*repository.KnowledgeStats, error) {
	if _, err := s.repo.GetKB(projectID, kbID); err != nil {
		return nil, err
	}
	return s.repo.Stats(kbID)
}

func (s *KnowledgeService) Graph(projectID, kbID uint64) (*KnowledgeGraph, error) {
	if _, err := s.repo.GetKB(projectID, kbID); err != nil {
		return nil, err
	}
	rows, err := s.repo.GraphRows(kbID)
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]KnowledgeGraphNode)
	edges := make([]KnowledgeGraphEdge, 0)
	seenChunks := make(map[uint64]struct{})
	seenEdges := make(map[string]struct{})
	for _, row := range rows {
		docID := fmt.Sprintf("doc-%d", row.DocID)
		chunkID := fmt.Sprintf("chunk-%d", row.ChunkID)
		nodes[docID] = KnowledgeGraphNode{ID: docID, Name: firstDocString(row.DocTitle, fmt.Sprintf("Doc %d", row.DocID)), Type: "document", Category: "document", Value: 12, Meta: map[string]any{"doc_id": row.DocID}}
		if _, exists := seenChunks[row.ChunkID]; !exists {
			seenChunks[row.ChunkID] = struct{}{}
			nodes[chunkID] = KnowledgeGraphNode{
				ID:       chunkID,
				Name:     fmt.Sprintf("Chunk %d", row.ChunkIndex+1),
				Type:     "chunk",
				Category: "chunk",
				Value:    6,
				Meta:     map[string]any{"chunk_id": row.ChunkID, "doc_id": row.DocID, "text": truncateGraphText(row.ChunkText, 500)},
			}
			edges = appendGraphEdge(edges, seenEdges, KnowledgeGraphEdge{Source: docID, Target: chunkID, Type: "contains", Score: 1})
			for _, tag := range extractKeywords(row.ChunkText, 4) {
				tagID := "tag-" + tag
				nodes[tagID] = KnowledgeGraphNode{ID: tagID, Name: tag, Type: "tag", Category: "tag", Value: 4, Meta: map[string]any{"tag": tag}}
				edges = appendGraphEdge(edges, seenEdges, KnowledgeGraphEdge{Source: chunkID, Target: tagID, Type: "tag", Score: 1})
			}
		}
		if row.TargetID != nil && row.RelationType != nil {
			score := 0.0
			if row.Score != nil {
				score = *row.Score
			}
			edges = appendGraphEdge(edges, seenEdges, KnowledgeGraphEdge{Source: chunkID, Target: fmt.Sprintf("chunk-%d", *row.TargetID), Type: *row.RelationType, Score: score})
		}
	}
	list := make([]KnowledgeGraphNode, 0, len(nodes))
	for _, node := range nodes {
		list = append(list, node)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return &KnowledgeGraph{Nodes: list, Edges: edges}, nil
}

func appendGraphEdge(edges []KnowledgeGraphEdge, seen map[string]struct{}, edge KnowledgeGraphEdge) []KnowledgeGraphEdge {
	key := edge.Source + "->" + edge.Target + ":" + edge.Type
	if _, exists := seen[key]; exists {
		return edges
	}
	seen[key] = struct{}{}
	return append(edges, edge)
}

func truncateGraphText(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

func (s *KnowledgeService) RetrieveContextForGeneration(ctx context.Context, projectID uint64, query string, workflow *model.Skill) (string, error) {
	if !s.configService.Current().Enabled || s.vectorStore == nil || !workflowRAGEnabled(workflow) {
		return "", nil
	}
	kbs, err := s.repo.ListActiveKBs(projectID)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0)
	for _, kb := range kbs {
		results, err := s.pipeline.Retrieve(ctx, projectID, kb.ID, query, s.configService.Current().TopK, nil)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("知识库检索失败，跳过当前知识库", zap.Uint64("project_id", projectID), zap.Uint64("kb_id", kb.ID), zap.Error(err))
			}
			continue
		}
		for _, item := range results {
			parts = append(parts, fmt.Sprintf("- 来源: %s / score %.4f\n%s", kb.Name, item.Score, strings.TrimSpace(item.Content)))
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "## RAG 知识库检索上下文\n请优先参考以下项目规范、历史用例或最佳实践，但如与仓库事实冲突，以真实代码为准。\n" + strings.Join(parts, "\n\n"), nil
}

func (s *KnowledgeService) rebuildRelations(kbID uint64) error {
	chunks, err := s.repo.ListChunksByKB(kbID)
	if err != nil {
		return err
	}
	if err := s.repo.ClearRelationsByKB(kbID); err != nil {
		return err
	}
	threshold := s.configService.Current().SimilarityThreshold
	relations := make([]model.ChunkRelation, 0)
	for i := range chunks {
		for j := i + 1; j < len(chunks); j++ {
			score := lexicalSimilarity(chunks[i].ChunkText, chunks[j].ChunkText)
			if score >= threshold {
				relations = append(relations, model.ChunkRelation{SourceChunkID: chunks[i].ID, TargetChunkID: chunks[j].ID, RelationType: "similar", Score: score})
				relations = append(relations, model.ChunkRelation{SourceChunkID: chunks[j].ID, TargetChunkID: chunks[i].ID, RelationType: "similar", Score: score})
			}
		}
	}
	return s.repo.UpsertRelations(relations)
}

func workflowRAGEnabled(workflow *model.Skill) bool {
	if workflow == nil || len(workflow.ConfigJSON) == 0 {
		return true
	}
	var cfg map[string]any
	if err := json.Unmarshal(workflow.ConfigJSON, &cfg); err != nil {
		return true
	}
	value, exists := cfg["rag_enabled"]
	if !exists {
		return true
	}
	enabled, ok := value.(bool)
	if !ok {
		return true
	}
	return enabled
}

func firstDocString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func isRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
