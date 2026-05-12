package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"auto-test-flow/internal/model"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

type RAGPipeline struct {
	configService *KnowledgeBaseConfigService
	vectorStore   VectorStore
}

func NewRAGPipeline(configService *KnowledgeBaseConfigService, vectorStore VectorStore) *RAGPipeline {
	return &RAGPipeline{configService: configService, vectorStore: vectorStore}
}

func (p *RAGPipeline) LoadSource(ctx context.Context, sourceType, sourcePath, content string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "manual", "markdown", "code", "":
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("文档内容不能为空")
		}
		return content, nil
	case "url":
		if strings.TrimSpace(sourcePath) == "" {
			return "", fmt.Errorf("URL 不能为空")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(sourcePath), nil)
		if err != nil {
			return "", err
		}
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("URL 返回错误状态码: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		if err != nil {
			return "", err
		}
		return stripHTML(string(body)), nil
	default:
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("暂不支持的文档源类型或内容为空: %s", sourceType)
		}
		return content, nil
	}
}

func (p *RAGPipeline) Split(ctx context.Context, doc *model.KnowledgeDocument, kb *model.KnowledgeBase) ([]model.KnowledgeChunk, error) {
	chunkSize := kb.ChunkSize
	if chunkSize <= 0 {
		chunkSize = p.configService.Current().ChunkSize
	}
	overlap := kb.ChunkOverlap
	if overlap < 0 || overlap >= chunkSize {
		overlap = p.configService.Current().ChunkOverlap
	}
	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   chunkSize,
		OverlapSize: overlap,
		Separators:  []string{"\n\n", "\n", "。", ".", " ", ""},
		LenFunc:     func(s string) int { return len([]rune(s)) },
		IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
			return fmt.Sprintf("%s_%d", originalID, splitIndex)
		},
	})
	if err != nil {
		return nil, err
	}
	input := []*schema.Document{{
		ID:      fmt.Sprintf("doc_%d", doc.ID),
		Content: doc.Content,
		MetaData: map[string]any{
			"doc_id":      doc.ID,
			"source_type": doc.SourceType,
			"source_path": doc.SourcePath,
			"title":       doc.Title,
		},
	}}
	docs, err := splitter.Transform(ctx, input)
	if err != nil {
		return nil, err
	}
	chunks := make([]model.KnowledgeChunk, 0, len(docs))
	for i, item := range docs {
		text := strings.TrimSpace(item.Content)
		if text == "" {
			continue
		}
		meta := map[string]any{
			"doc_id":      doc.ID,
			"chunk_index": i,
			"title":       doc.Title,
			"source_type": doc.SourceType,
			"source_path": doc.SourcePath,
			"tags":        extractKeywords(text, 6),
		}
		metaBytes, _ := json.Marshal(meta)
		chunks = append(chunks, model.KnowledgeChunk{
			DocID:      doc.ID,
			ChunkIndex: i,
			ChunkText:  text,
			Metadata:   model.JSON(metaBytes),
		})
	}
	return chunks, nil
}

func (p *RAGPipeline) StoreChunks(ctx context.Context, projectID, kbID uint64, chunks []model.KnowledgeChunk) ([]string, error) {
	if p.vectorStore == nil {
		return nil, fmt.Errorf("RAG 未启用或向量存储未初始化")
	}
	docs := make([]VectorDocument, 0, len(chunks))
	for _, chunk := range chunks {
		vectorID := vectorIDForChunk(chunk.ID)
		meta := map[string]any{
			"chunk_id":          chunk.ID,
			"doc_id":            chunk.DocID,
			"chunk_index":       chunk.ChunkIndex,
			"project_id":        projectID,
			"knowledge_base_id": kbID,
		}
		if len(chunk.Metadata) > 0 {
			var chunkMeta map[string]any
			if err := json.Unmarshal(chunk.Metadata, &chunkMeta); err == nil {
				for k, v := range chunkMeta {
					if _, exists := meta[k]; !exists {
						meta[k] = v
					}
				}
			}
		}
		docs = append(docs, VectorDocument{
			ID:       vectorID,
			Content:  chunk.ChunkText,
			Metadata: meta,
		})
	}
	return p.vectorStore.Store(ctx, projectID, kbID, docs)
}

func (p *RAGPipeline) Retrieve(ctx context.Context, projectID, kbID uint64, query string, topK int, keywords []string) ([]VectorSearchResult, error) {
	if p.vectorStore == nil {
		return nil, fmt.Errorf("RAG 未启用或向量存储未初始化")
	}
	cfg := p.configService.Current()
	return p.vectorStore.Search(ctx, query, VectorSearchOptions{
		ProjectID:           projectID,
		KnowledgeBaseID:     kbID,
		TopK:                topK,
		SimilarityThreshold: cfg.SimilarityThreshold,
		Keywords:            keywords,
	})
}

func vectorIDForChunk(chunkID uint64) string {
	return fmt.Sprintf("chunk_%d", chunkID)
}

func extractChunkIDFromVectorID(id string) uint64 {
	var chunkID uint64
	_, _ = fmt.Sscanf(id, "chunk_%d", &chunkID)
	return chunkID
}

func stripHTML(input string) string {
	reScript := regexp.MustCompile(`(?is)<(script|style).*?</\1>`)
	input = reScript.ReplaceAllString(input, " ")
	reTag := regexp.MustCompile(`(?s)<[^>]+>`)
	input = reTag.ReplaceAllString(input, " ")
	reSpace := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(reSpace.ReplaceAllString(input, " "))
}

func extractKeywords(text string, limit int) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_\-]{3,}|[\p{Han}]{2,}`)
	words := re.FindAllString(strings.ToLower(text), -1)
	counts := make(map[string]int)
	for _, word := range words {
		if len([]rune(word)) > 40 {
			continue
		}
		counts[word]++
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for word, count := range counts {
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	result := make([]string, 0, len(pairs))
	for _, item := range pairs {
		result = append(result, item.word)
	}
	return result
}

func lexicalSimilarity(a, b string) float64 {
	ka := extractKeywords(a, 32)
	kb := extractKeywords(b, 32)
	if len(ka) == 0 || len(kb) == 0 {
		return 0
	}
	set := make(map[string]bool, len(ka))
	for _, item := range ka {
		set[item] = true
	}
	intersect := 0
	for _, item := range kb {
		if set[item] {
			intersect++
		}
		set[item] = true
	}
	return float64(intersect) / float64(len(set))
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
