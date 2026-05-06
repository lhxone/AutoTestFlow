package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	openaiembedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	milvusindexer "github.com/cloudwego/eino-ext/components/indexer/milvus2"
	milvusretriever "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type VectorDocument struct {
	ID       string
	Content  string
	Metadata map[string]any
}

type VectorSearchResult struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

type VectorSearchOptions struct {
	ProjectID           uint64
	KnowledgeBaseID     uint64
	TopK                int
	SimilarityThreshold float64
	Keywords            []string
}

type VectorStore interface {
	Store(ctx context.Context, projectID, kbID uint64, docs []VectorDocument) ([]string, error)
	Search(ctx context.Context, query string, opts VectorSearchOptions) ([]VectorSearchResult, error)
	DeleteByIDs(ctx context.Context, projectID, kbID uint64, ids []string) error
	DeleteKnowledgeBase(ctx context.Context, projectID, kbID uint64) error
	Close(ctx context.Context) error
}

type MilvusVectorStore struct {
	cfg      KnowledgeBaseConfig
	client   *milvusclient.Client
	embedder *openaiembedding.Embedder
}

func NewVectorStore(ctx context.Context, cfg KnowledgeBaseConfig) (VectorStore, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.ToLower(strings.TrimSpace(cfg.VectorStoreType)) != "milvus" {
		return nil, fmt.Errorf("不支持的向量存储类型: %s", cfg.VectorStoreType)
	}
	aiCfg := LoadAIConfig()
	if strings.TrimSpace(aiCfg.APIKey) == "" {
		return nil, fmt.Errorf("知识库已启用，但 AI API Key 为空，无法创建 Embedding 客户端")
	}
	dim := cfg.EmbeddingDimension
	embedder, err := openaiembedding.NewEmbedder(ctx, &openaiembedding.EmbeddingConfig{
		APIKey:     aiCfg.APIKey,
		BaseURL:    strings.TrimRight(aiCfg.BaseURL, "/"),
		Model:      cfg.EmbeddingModel,
		Dimensions: &dim,
		Timeout:    60 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Embedding 客户端失败: %w", err)
	}
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: fmt.Sprintf("%s:%d", cfg.VectorStoreHost, cfg.VectorStorePort),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus 失败: %w", err)
	}
	return &MilvusVectorStore{cfg: cfg, client: client, embedder: embedder}, nil
}

func (s *MilvusVectorStore) Store(ctx context.Context, projectID, kbID uint64, docs []VectorDocument) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("向量存储未初始化")
	}
	if len(docs) == 0 {
		return []string{}, nil
	}
	partition := knowledgePartition(projectID, kbID)
	indexer, err := milvusindexer.NewIndexer(ctx, &milvusindexer.IndexerConfig{
		Client:     s.client,
		Collection: s.cfg.VectorStoreCollection,
		Embedding:  s.embedder,
		Vector: &milvusindexer.VectorConfig{
			Dimension:  int64(s.cfg.EmbeddingDimension),
			MetricType: milvusindexer.COSINE,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := s.ensurePartition(ctx, partition); err != nil {
		return nil, err
	}
	einoDocs := make([]*schema.Document, 0, len(docs))
	for _, item := range docs {
		meta := item.Metadata
		if meta == nil {
			meta = map[string]any{}
		}
		meta["project_id"] = projectID
		meta["knowledge_base_id"] = kbID
		einoDocs = append(einoDocs, &schema.Document{
			ID:       item.ID,
			Content:  item.Content,
			MetaData: meta,
		})
	}
	return indexer.Store(ctx, einoDocs, milvusindexer.WithPartition(partition))
}

func (s *MilvusVectorStore) Search(ctx context.Context, query string, opts VectorSearchOptions) ([]VectorSearchResult, error) {
	if s == nil {
		return nil, fmt.Errorf("向量存储未初始化")
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = s.cfg.TopK
	}
	retriever, err := milvusretriever.NewRetriever(ctx, &milvusretriever.RetrieverConfig{
		Client:       s.client,
		Collection:   s.cfg.VectorStoreCollection,
		Partitions:   []string{knowledgePartition(opts.ProjectID, opts.KnowledgeBaseID)},
		Embedding:    s.embedder,
		TopK:         topK,
		SearchMode:   search_mode.NewApproximate(milvusretriever.COSINE),
		OutputFields: []string{"id", "content", "metadata"},
	})
	if err != nil {
		return nil, err
	}
	docs, err := retriever.Retrieve(ctx, query)
	if err != nil {
		return nil, err
	}
	results := make([]VectorSearchResult, 0, len(docs))
	for _, doc := range docs {
		score := doc.Score()
		if opts.SimilarityThreshold > 0 && score < opts.SimilarityThreshold {
			continue
		}
		if !matchesKeywords(doc.Content, opts.Keywords) {
			continue
		}
		results = append(results, VectorSearchResult{
			ID:       doc.ID,
			Content:  doc.Content,
			Score:    score,
			Metadata: doc.MetaData,
		})
	}
	return results, nil
}

func (s *MilvusVectorStore) DeleteByIDs(ctx context.Context, projectID, kbID uint64, ids []string) error {
	if s == nil || len(ids) == 0 {
		return nil
	}
	_, err := s.client.Delete(ctx, milvusclient.NewDeleteOption(s.cfg.VectorStoreCollection).
		WithPartition(knowledgePartition(projectID, kbID)).
		WithStringIDs("id", ids))
	return err
}

func (s *MilvusVectorStore) DeleteKnowledgeBase(ctx context.Context, projectID, kbID uint64) error {
	if s == nil {
		return nil
	}
	partition := knowledgePartition(projectID, kbID)
	has, err := s.client.HasPartition(ctx, milvusclient.NewHasPartitionOption(s.cfg.VectorStoreCollection, partition))
	if err != nil || !has {
		return err
	}
	return s.client.DropPartition(ctx, milvusclient.NewDropPartitionOption(s.cfg.VectorStoreCollection, partition))
}

func (s *MilvusVectorStore) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close(ctx)
}

func (s *MilvusVectorStore) ensurePartition(ctx context.Context, partition string) error {
	has, err := s.client.HasPartition(ctx, milvusclient.NewHasPartitionOption(s.cfg.VectorStoreCollection, partition))
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return s.client.CreatePartition(ctx, milvusclient.NewCreatePartitionOption(s.cfg.VectorStoreCollection, partition))
}

func knowledgePartition(projectID, kbID uint64) string {
	return fmt.Sprintf("p%d_kb%d", projectID, kbID)
}

func matchesKeywords(content string, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}
	lower := strings.ToLower(content)
	for _, keyword := range keywords {
		if trimmed := strings.ToLower(strings.TrimSpace(keyword)); trimmed != "" && strings.Contains(lower, trimmed) {
			return true
		}
	}
	return false
}
