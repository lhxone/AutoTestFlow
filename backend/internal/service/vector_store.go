package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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
	apiKey := strings.TrimSpace(cfg.EmbeddingAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("知识库已启用，但 knowledge_base.embedding.api_key 为空，无法创建 Embedding 客户端")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.EmbeddingBaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("知识库已启用，但 knowledge_base.embedding.base_url 为空，无法创建 Embedding 客户端")
	}
	dim := cfg.EmbeddingDimension
	embedder, err := openaiembedding.NewEmbedder(ctx, &openaiembedding.EmbeddingConfig{
		APIKey:     apiKey,
		BaseURL:    baseURL,
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
		return nil, fmt.Errorf("连接 Milvus 失败 (%s:%d): %w", cfg.VectorStoreHost, cfg.VectorStorePort, err)
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
	batchSize := s.cfg.EmbeddingBatchSize
	if batchSize <= 0 || batchSize > 64 {
		batchSize = 64
	}
	ids := make([]string, 0, len(docs))
	for start := 0; start < len(docs); start += batchSize {
		end := start + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		einoDocs := make([]*schema.Document, 0, end-start)
		for _, item := range docs[start:end] {
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
		storedIDs, err := indexer.Store(ctx, einoDocs, milvusindexer.WithPartition(partition))
		if err != nil {
			return ids, err
		}
		ids = append(ids, storedIDs...)
	}
	flushTask, err := s.client.Flush(ctx, milvusclient.NewFlushOption(s.cfg.VectorStoreCollection))
	if err != nil {
		flushTask, err = s.retryFlush(ctx)
		if err != nil {
			return ids, fmt.Errorf("flush Milvus collection 失败: %w", err)
		}
	}
	if err := flushTask.Await(ctx); err != nil {
		return ids, fmt.Errorf("等待 Milvus flush 完成失败: %w", err)
	}
	return ids, nil
}

func (s *MilvusVectorStore) retryFlush(ctx context.Context) (*milvusclient.FlushTask, error) {
	const maxRetries = 6
	delay := 2 * time.Second
	lastErr := error(nil)
	for i := 0; i < maxRetries; i++ {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		task, err := s.client.Flush(ctx, milvusclient.NewFlushOption(s.cfg.VectorStoreCollection))
		if err == nil {
			return task, nil
		}
		lastErr = err
		if !isMilvusRateLimitError(err) {
			return nil, err
		}
		delay = nextMilvusFlushRetryDelay(err, delay)
	}
	return nil, lastErr
}

func isMilvusRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "ratelimiter") || strings.Contains(lower, "rate limit exceeded")
}

func nextMilvusFlushRetryDelay(err error, fallback time.Duration) time.Duration {
	if err == nil {
		return fallback
	}
	re := regexp.MustCompile(`rate=([0-9]+(?:\.[0-9]+)?)`)
	matches := re.FindStringSubmatch(strings.ToLower(err.Error()))
	if len(matches) < 2 {
		if fallback < 20*time.Second {
			return fallback * 2
		}
		return fallback
	}
	rate, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil || rate <= 0 {
		return fallback
	}
	waitSeconds := int(1.0/rate) + 1
	wait := time.Duration(waitSeconds) * time.Second
	if wait < 3*time.Second {
		wait = 3 * time.Second
	}
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}
	return wait
}

func (s *MilvusVectorStore) Search(ctx context.Context, query string, opts VectorSearchOptions) ([]VectorSearchResult, error) {
	if s == nil {
		return nil, fmt.Errorf("向量存储未初始化")
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = s.cfg.TopK
	}
	partition := knowledgePartition(opts.ProjectID, opts.KnowledgeBaseID)
	hasPartition, err := s.client.HasPartition(ctx, milvusclient.NewHasPartitionOption(s.cfg.VectorStoreCollection, partition))
	if err != nil {
		return nil, fmt.Errorf("检查 Milvus partition 失败 (%s): %w", partition, err)
	}
	if !hasPartition {
		return []VectorSearchResult{}, nil
	}
	retriever, err := milvusretriever.NewRetriever(ctx, &milvusretriever.RetrieverConfig{
		Client:       s.client,
		Collection:   s.cfg.VectorStoreCollection,
		Partitions:   []string{partition},
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
