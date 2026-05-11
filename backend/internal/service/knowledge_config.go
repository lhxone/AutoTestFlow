package service

import (
	"strconv"
	"strings"
	"sync"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const (
	kbKeyEnabled             = "enabled"
	kbKeyVectorStoreType     = "vector_store.type"
	kbKeyVectorStoreHost     = "vector_store.host"
	kbKeyVectorStorePort     = "vector_store.port"
	kbKeyVectorStoreColl     = "vector_store.collection"
	kbKeyEmbeddingProvider   = "embedding.provider"
	kbKeyEmbeddingAPIKey     = "embedding.api_key"
	kbKeyEmbeddingBaseURL    = "embedding.base_url"
	kbKeyEmbeddingModel      = "embedding.model"
	kbKeyEmbeddingDimension  = "embedding.dimension"
	kbKeyEmbeddingBatchSize  = "embedding.batch_size"
	kbKeyChunkSize           = "chunk_size"
	kbKeyChunkOverlap        = "chunk_overlap"
	kbKeyTopK                = "top_k"
	kbKeySimilarityThreshold = "similarity_threshold"
)

type KnowledgeBaseConfig struct {
	Enabled               bool    `json:"enabled"`
	VectorStoreType       string  `json:"vector_store_type"`
	VectorStoreHost       string  `json:"vector_store_host"`
	VectorStorePort       int     `json:"vector_store_port"`
	VectorStoreCollection string  `json:"vector_store_collection"`
	EmbeddingProvider     string  `json:"embedding_provider"`
	EmbeddingAPIKey       string  `json:"embedding_api_key"`
	EmbeddingBaseURL      string  `json:"embedding_base_url"`
	EmbeddingModel        string  `json:"embedding_model"`
	EmbeddingDimension    int     `json:"embedding_dimension"`
	EmbeddingBatchSize    int     `json:"embedding_batch_size"`
	ChunkSize             int     `json:"chunk_size"`
	ChunkOverlap          int     `json:"chunk_overlap"`
	TopK                  int     `json:"top_k"`
	SimilarityThreshold   float64 `json:"similarity_threshold"`
}

type KnowledgeBaseConfigService struct {
	repo   *repository.KnowledgeConfigRepo
	logger *zap.Logger
	mu     sync.RWMutex
	cache  KnowledgeBaseConfig
}

var DefaultKnowledgeConfig *KnowledgeBaseConfigService

func InitKnowledgeBaseConfig(logger *zap.Logger) *KnowledgeBaseConfigService {
	DefaultKnowledgeConfig = NewKnowledgeBaseConfigService(logger)
	if _, err := DefaultKnowledgeConfig.Refresh(); err != nil && logger != nil {
		logger.Warn("加载知识库配置失败", zap.Error(err))
	}
	return DefaultKnowledgeConfig
}

func NewKnowledgeBaseConfigService(logger *zap.Logger) *KnowledgeBaseConfigService {
	return &KnowledgeBaseConfigService{
		repo:   repository.NewKnowledgeConfigRepo(),
		logger: logger,
		cache:  defaultKnowledgeBaseConfig(),
	}
}

func (s *KnowledgeBaseConfigService) Current() KnowledgeBaseConfig {
	if s == nil {
		return defaultKnowledgeBaseConfig()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache
}

func (s *KnowledgeBaseConfigService) Refresh() (KnowledgeBaseConfig, error) {
	cfg := defaultKnowledgeBaseConfig()
	settings, err := s.repo.List()
	if err != nil {
		return cfg, err
	}
	values := make(map[string]string, len(settings))
	for _, item := range settings {
		values[item.Key] = item.Value
	}
	cfg.Enabled = parseKBBool(kbValue(values, kbKeyEnabled), cfg.Enabled)
	cfg.VectorStoreType = firstKBString(kbValue(values, kbKeyVectorStoreType), cfg.VectorStoreType)
	cfg.VectorStoreHost = firstKBString(kbValue(values, kbKeyVectorStoreHost), cfg.VectorStoreHost)
	cfg.VectorStorePort = parseKBInt(kbValue(values, kbKeyVectorStorePort), cfg.VectorStorePort)
	cfg.VectorStoreCollection = firstKBString(kbValue(values, kbKeyVectorStoreColl), cfg.VectorStoreCollection)
	cfg.EmbeddingProvider = firstKBString(kbValue(values, kbKeyEmbeddingProvider), cfg.EmbeddingProvider)
	cfg.EmbeddingAPIKey = firstKBString(kbValue(values, kbKeyEmbeddingAPIKey), cfg.EmbeddingAPIKey)
	cfg.EmbeddingBaseURL = firstKBString(kbValue(values, kbKeyEmbeddingBaseURL), cfg.EmbeddingBaseURL)
	cfg.EmbeddingModel = firstKBString(kbValue(values, kbKeyEmbeddingModel), cfg.EmbeddingModel)
	cfg.EmbeddingDimension = parseKBInt(kbValue(values, kbKeyEmbeddingDimension), cfg.EmbeddingDimension)
	cfg.EmbeddingBatchSize = parseKBInt(kbValue(values, kbKeyEmbeddingBatchSize), cfg.EmbeddingBatchSize)
	cfg.ChunkSize = parseKBInt(kbValue(values, kbKeyChunkSize), cfg.ChunkSize)
	cfg.ChunkOverlap = parseKBInt(kbValue(values, kbKeyChunkOverlap), cfg.ChunkOverlap)
	cfg.TopK = parseKBInt(kbValue(values, kbKeyTopK), cfg.TopK)
	cfg.SimilarityThreshold = parseKBFloat(kbValue(values, kbKeySimilarityThreshold), cfg.SimilarityThreshold)

	s.mu.Lock()
	s.cache = cfg
	s.mu.Unlock()
	return cfg, nil
}

func (s *KnowledgeBaseConfigService) Save(cfg KnowledgeBaseConfig, operatorID uint64) error {
	normalized := normalizeKnowledgeBaseConfig(cfg)
	settings := []model.SystemSetting{
		{Key: kbKeyEnabled, Value: strconv.FormatBool(normalized.Enabled), Description: "是否启用 RAG 知识库", UpdatedBy: &operatorID},
		{Key: kbKeyVectorStoreType, Value: normalized.VectorStoreType, Description: "向量存储类型", UpdatedBy: &operatorID},
		{Key: kbKeyVectorStoreHost, Value: normalized.VectorStoreHost, Description: "Milvus 地址", UpdatedBy: &operatorID},
		{Key: kbKeyVectorStorePort, Value: strconv.Itoa(normalized.VectorStorePort), Description: "Milvus 端口", UpdatedBy: &operatorID},
		{Key: kbKeyVectorStoreColl, Value: normalized.VectorStoreCollection, Description: "Milvus Collection", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingProvider, Value: normalized.EmbeddingProvider, Description: "Embedding 服务类型", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingAPIKey, Value: normalized.EmbeddingAPIKey, Encrypted: 1, Description: "Embedding API Key", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingBaseURL, Value: normalized.EmbeddingBaseURL, Description: "Embedding OpenAI 兼容 Base URL", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingModel, Value: normalized.EmbeddingModel, Description: "Embedding 模型名", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingDimension, Value: strconv.Itoa(normalized.EmbeddingDimension), Description: "Embedding 向量维度", UpdatedBy: &operatorID},
		{Key: kbKeyEmbeddingBatchSize, Value: strconv.Itoa(normalized.EmbeddingBatchSize), Description: "Embedding 批大小", UpdatedBy: &operatorID},
		{Key: kbKeyChunkSize, Value: strconv.Itoa(normalized.ChunkSize), Description: "默认 chunk 大小", UpdatedBy: &operatorID},
		{Key: kbKeyChunkOverlap, Value: strconv.Itoa(normalized.ChunkOverlap), Description: "默认 chunk 重叠", UpdatedBy: &operatorID},
		{Key: kbKeyTopK, Value: strconv.Itoa(normalized.TopK), Description: "默认检索数量", UpdatedBy: &operatorID},
		{Key: kbKeySimilarityThreshold, Value: strconv.FormatFloat(normalized.SimilarityThreshold, 'f', -1, 64), Description: "相似度阈值", UpdatedBy: &operatorID},
	}
	if err := s.repo.BatchUpsert(settings); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache = normalized
	s.mu.Unlock()
	return nil
}

func defaultKnowledgeBaseConfig() KnowledgeBaseConfig {
	return KnowledgeBaseConfig{
		Enabled:               false,
		VectorStoreType:       "milvus",
		VectorStoreHost:       "milvus-standalone",
		VectorStorePort:       19530,
		VectorStoreCollection: "autotestflow_knowledge",
		EmbeddingProvider:     "openai_compatible",
		EmbeddingAPIKey:       "",
		EmbeddingBaseURL:      "https://api.openai.com/v1",
		EmbeddingModel:        "text-embedding-3-small",
		EmbeddingDimension:    1536,
		EmbeddingBatchSize:    16,
		ChunkSize:             500,
		ChunkOverlap:          50,
		TopK:                  5,
		SimilarityThreshold:   0.75,
	}
}

func normalizeKnowledgeBaseConfig(cfg KnowledgeBaseConfig) KnowledgeBaseConfig {
	def := defaultKnowledgeBaseConfig()
	cfg.VectorStoreType = firstKBString(cfg.VectorStoreType, def.VectorStoreType)
	cfg.VectorStoreHost = firstKBString(cfg.VectorStoreHost, def.VectorStoreHost)
	if cfg.VectorStorePort <= 0 {
		cfg.VectorStorePort = def.VectorStorePort
	}
	cfg.VectorStoreCollection = firstKBString(cfg.VectorStoreCollection, def.VectorStoreCollection)
	cfg.EmbeddingProvider = firstKBString(cfg.EmbeddingProvider, def.EmbeddingProvider)
	cfg.EmbeddingAPIKey = strings.TrimSpace(cfg.EmbeddingAPIKey)
	cfg.EmbeddingBaseURL = firstKBString(strings.TrimRight(strings.TrimSpace(cfg.EmbeddingBaseURL), "/"), def.EmbeddingBaseURL)
	cfg.EmbeddingModel = firstKBString(cfg.EmbeddingModel, def.EmbeddingModel)
	if cfg.EmbeddingDimension <= 0 {
		cfg.EmbeddingDimension = def.EmbeddingDimension
	}
	if cfg.EmbeddingBatchSize <= 0 {
		cfg.EmbeddingBatchSize = def.EmbeddingBatchSize
	}
	if cfg.EmbeddingBatchSize > 64 {
		cfg.EmbeddingBatchSize = 64
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = def.ChunkSize
	}
	if cfg.ChunkOverlap < 0 || cfg.ChunkOverlap >= cfg.ChunkSize {
		cfg.ChunkOverlap = def.ChunkOverlap
	}
	if cfg.TopK <= 0 {
		cfg.TopK = def.TopK
	}
	if cfg.SimilarityThreshold <= 0 || cfg.SimilarityThreshold > 1 {
		cfg.SimilarityThreshold = def.SimilarityThreshold
	}
	return cfg
}

func kbValue(values map[string]string, key string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return values["knowledge_base."+key]
}

func firstKBString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func parseKBBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseKBInt(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func parseKBFloat(value string, fallback float64) float64 {
	if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}
