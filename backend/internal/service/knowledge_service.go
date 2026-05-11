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
	agentRepo     *repository.AgentRepo
	configService *KnowledgeBaseConfigService
	pipeline      *RAGPipeline
	vectorStore   VectorStore
	einoRuntime   *EinoGenTestRuntime
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

type KnowledgeChatMessage struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type KnowledgeChatRequest struct {
	ProjectID uint64                 `json:"project_id" binding:"required"`
	Query     string                 `json:"query" binding:"required"`
	TopK      int                    `json:"top_k"`
	Keywords  []string               `json:"keywords"`
	AgentID   *uint64                `json:"agent_id"`
	Messages  []KnowledgeChatMessage `json:"messages"`
}

type KnowledgeChatResponse struct {
	Answer  string               `json:"answer"`
	Sources []VectorSearchResult `json:"sources"`
	Agent   *KnowledgeChatAgent  `json:"agent,omitempty"`
}

type KnowledgeChatAgent struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
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
		agentRepo:     repository.NewAgentRepo(),
		configService: cfgSvc,
		vectorStore:   store,
		pipeline:      NewRAGPipeline(cfgSvc, store),
		einoRuntime:   NewEinoGenTestRuntime(logger),
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
	if err := s.ensureVectorStore(ctx); err != nil {
		return err
	}
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
	if err := s.ensureVectorStore(ctx); err != nil {
		return err
	}
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
	if err := s.ensureVectorStore(ctx); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetKB(req.ProjectID, kbID); err != nil {
		return nil, err
	}
	return s.retrieveWithRetry(ctx, req.ProjectID, kbID, strings.TrimSpace(req.Query), req.TopK, req.Keywords)
}

func (s *KnowledgeService) Chat(ctx context.Context, kbID uint64, req KnowledgeChatRequest) (*KnowledgeChatResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query 不能为空")
	}
	if err := s.ensureVectorStore(ctx); err != nil {
		return nil, err
	}
	kb, err := s.repo.GetKB(req.ProjectID, kbID)
	if err != nil {
		return nil, err
	}
	results, err := s.retrieveWithRetry(ctx, req.ProjectID, kbID, query, req.TopK, req.Keywords)
	if err != nil {
		return nil, err
	}

	agent, err := s.resolveKnowledgeChatAgent(req.AgentID)
	if err != nil {
		return nil, err
	}
	execCfg := ResolveAgentExecutionConfig(agent)
	if strings.TrimSpace(execCfg.APIKey) == "" {
		return nil, fmt.Errorf("Agent %s 未配置可用 API Key", safeAgentName(agent))
	}
	if execCfg.MaxTokens <= 0 || execCfg.MaxTokens > 1536 {
		execCfg.MaxTokens = 1536
	}

	history := []runtimeMessage{
		{Role: "system", Text: buildKnowledgeChatSystemPrompt(kb, results)},
	}
	history = append(history, normalizeKnowledgeChatHistory(req.Messages, query)...)
	history = append(history, runtimeMessage{Role: "user", Text: query})

	runtime := s.einoRuntime
	if runtime == nil {
		runtime = NewEinoGenTestRuntime(s.logger)
	}
	chatModel, err := runtime.newEinoChatModel(ctx, execCfg, nil)
	if err != nil {
		return nil, err
	}
	msg, err := chatModel.Generate(ctx, runtimeHistoryToEinoMessages(history))
	if err != nil {
		return nil, err
	}
	answer := ""
	if msg != nil {
		answer = strings.TrimSpace(msg.Content)
	}
	if answer == "" {
		answer = "未生成有效回答。"
	}
	return &KnowledgeChatResponse{
		Answer:  answer,
		Sources: results,
		Agent: &KnowledgeChatAgent{
			ID:       agent.ID,
			Name:     agent.Name,
			Provider: execCfg.Provider,
			Model:    execCfg.Model,
		},
	}, nil
}

func (s *KnowledgeService) resolveKnowledgeChatAgent(agentID *uint64) (*model.Agent, error) {
	if agentID != nil && *agentID > 0 {
		return s.agentRepo.GetByID(*agentID)
	}
	if agent, err := s.agentRepo.GetDefaultActive(); err == nil {
		return agent, nil
	}
	return s.agentRepo.GetFirstActive()
}

func buildKnowledgeChatSystemPrompt(kb *model.KnowledgeBase, results []VectorSearchResult) string {
	var b strings.Builder
	b.WriteString("你是 AutoTestFlow 知识库问答助手。请基于给定的知识库检索上下文回答用户问题。\n")
	b.WriteString("要求：\n")
	b.WriteString("1. 优先引用检索上下文，不要编造不存在的内容。\n")
	b.WriteString("2. 如果上下文不足以回答，请明确说明缺少哪些信息，并给出可继续检索的建议。\n")
	b.WriteString("3. 使用中文，结构清晰，回答尽量控制在 800 字以内。\n")
	b.WriteString("4. 当用户询问流程图、时序图、架构图或状态流转时，优先输出一个简洁的 ```mermaid 代码块；节点文字应短，避免 HTML 标签。\n")
	if kb != nil {
		b.WriteString(fmt.Sprintf("\n当前知识库：%s\n", kb.Name))
		if strings.TrimSpace(kb.Description) != "" {
			b.WriteString(fmt.Sprintf("知识库说明：%s\n", strings.TrimSpace(kb.Description)))
		}
	}
	if len(results) == 0 {
		b.WriteString("\n检索上下文：未命中相关内容。\n")
		return b.String()
	}
	b.WriteString("\n检索上下文：\n")
	for index, item := range results {
		title := metadataString(item.Metadata, "title")
		if title == "" {
			title = fmt.Sprintf("Chunk %s", firstDocString(metadataString(item.Metadata, "chunk_id"), item.ID))
		}
		b.WriteString(fmt.Sprintf("\n[%d] %s，score %.4f\n", index+1, title, item.Score))
		b.WriteString(truncateKnowledgeChatContext(item.Content, 1200))
		b.WriteString("\n")
	}
	return b.String()
}

func truncateKnowledgeChatContext(value string, limit int) string {
	text := strings.TrimSpace(value)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "\n[该检索片段已截断]"
}

func normalizeKnowledgeChatHistory(messages []KnowledgeChatMessage, currentQuery string) []runtimeMessage {
	if len(messages) == 0 {
		return nil
	}
	for len(messages) > 0 {
		last := messages[len(messages)-1]
		if strings.EqualFold(strings.TrimSpace(last.Role), "user") && strings.TrimSpace(last.Content) == strings.TrimSpace(currentQuery) {
			messages = messages[:len(messages)-1]
			continue
		}
		break
	}
	const maxHistory = 12
	if len(messages) > maxHistory {
		messages = messages[len(messages)-maxHistory:]
	}
	history := make([]runtimeMessage, 0, len(messages))
	for _, item := range messages {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch role {
		case "user", "assistant":
			history = append(history, runtimeMessage{Role: role, Text: content})
		}
	}
	return history
}

func metadataString(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	value, exists := meta[key]
	if !exists || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (s *KnowledgeService) retrieveWithRetry(ctx context.Context, projectID, kbID uint64, query string, topK int, keywords []string) ([]VectorSearchResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		results, err := s.pipeline.Retrieve(ctx, projectID, kbID, query, topK, keywords)
		if err == nil {
			return results, nil
		}
		lastErr = err
		if !isTransientKnowledgeRetrieveError(err) || attempt == 2 {
			break
		}
		if s.logger != nil {
			s.logger.Warn("知识库检索失败，短暂重试", zap.Uint64("project_id", projectID), zap.Uint64("kb_id", kbID), zap.Int("attempt", attempt+1), zap.Error(err))
		}
		if err := sleepWithContext(ctx, time.Duration(attempt+1)*2*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func isTransientKnowledgeRetrieveError(err error) bool {
	if err == nil {
		return false
	}
	if isModelConnectionError(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, token := range []string{
		"502",
		"503",
		"504",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"too many requests",
		"rate limit",
	} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
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
	if err := s.ensureVectorStore(ctx); err != nil {
		return "", err
	}
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

func (s *KnowledgeService) ensureVectorStore(ctx context.Context) error {
	cfg := s.configService.Current()
	if !cfg.Enabled || s.vectorStore != nil {
		return nil
	}
	initCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	store, err := NewVectorStore(initCtx, cfg)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("按需初始化知识库向量存储失败", zap.Error(err))
		}
		return err
	}
	s.vectorStore = store
	s.pipeline = NewRAGPipeline(s.configService, store)
	return nil
}
