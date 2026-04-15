package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type AgentService struct {
	logger      *zap.Logger
	settingRepo *repository.SettingRepo
	httpClient  *http.Client
}

func NewAgentService(logger *zap.Logger) *AgentService {
	return &AgentService{
		logger:      logger,
		settingRepo: repository.NewSettingRepo(),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (s *AgentService) TestConnection(req *dto.TestAgentConnectionRequest) *dto.TestAgentConnectionResponse {
	start := time.Now()
	apiKey := s.resolveAPIKey(req)
	if strings.TrimSpace(apiKey) == "" {
		return &dto.TestAgentConnectionResponse{
			Success:  false,
			Message:  "未找到可用的 API Key，请填写 API Key，或先配置全局 AI API Key",
			Provider: req.ModelProvider,
			Model:    req.ModelName,
			BaseURL:  strings.TrimSpace(req.BaseURL),
		}
	}

	baseURL := s.resolveBaseURL(req)
	if baseURL == "" {
		return &dto.TestAgentConnectionResponse{
			Success:  false,
			Message:  "Base URL 不能为空",
			Provider: req.ModelProvider,
			Model:    req.ModelName,
		}
	}

	var (
		content string
		err     error
	)
	switch req.ModelProvider {
	case "claude":
		content, err = s.testClaude(baseURL, apiKey, req)
	case "openai", "zhipu", "custom":
		content, err = s.testOpenAICompatible(baseURL, apiKey, req)
	default:
		err = fmt.Errorf("不支持的模型提供商: %s", req.ModelProvider)
	}

	result := &dto.TestAgentConnectionResponse{
		Success:      err == nil,
		Message:      "连接成功",
		Provider:     req.ModelProvider,
		Model:        req.ModelName,
		BaseURL:      baseURL,
		LatencyMs:    time.Since(start).Milliseconds(),
		SampleOutput: truncateOutput(content, 200),
	}
	if err != nil {
		result.Message = err.Error()
		result.SampleOutput = ""
	}
	return result
}

func (s *AgentService) resolveAPIKey(req *dto.TestAgentConnectionRequest) string {
	if value := strings.TrimSpace(req.TestAPIKey); value != "" {
		return value
	}

	value := strings.TrimSpace(req.APIKeyRef)
	if value != "" {
		if looksLikeAPIKey(value) {
			return value
		}

		if envValue := strings.TrimSpace(os.Getenv(value)); envValue != "" {
			return envValue
		}

		normalized := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_", " ", "_").Replace(value))
		if envValue := strings.TrimSpace(os.Getenv(normalized)); envValue != "" {
			return envValue
		}

		for _, category := range []string{"agent", "ai", "secret", "secrets"} {
			if settingValue := strings.TrimSpace(s.settingRepo.GetValue(category, value)); settingValue != "" {
				return settingValue
			}
		}

		return value
	}

	if strings.EqualFold(LoadAIConfig().Provider, req.ModelProvider) {
		return strings.TrimSpace(LoadAIConfig().APIKey)
	}

	return ""
}

func (s *AgentService) resolveBaseURL(req *dto.TestAgentConnectionRequest) string {
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL != "" {
		return baseURL
	}

	switch req.ModelProvider {
	case "claude":
		return "https://api.anthropic.com"
	case "openai":
		return "https://api.openai.com"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	default:
		return ""
	}
}

func (s *AgentService) testClaude(baseURL, apiKey string, req *dto.TestAgentConnectionRequest) (string, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 || maxTokens > 64 {
		maxTokens = 32
	}

	requestBody := map[string]any{
		"model":      req.ModelName,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with PONG only."},
		},
	}

	body, _ := json.Marshal(requestBody)
	httpReq, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构建 Anthropic 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求 Anthropic 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Anthropic 返回错误 %d: %s", resp.StatusCode, truncateOutput(string(respBody), 300))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("Anthropic 返回空内容")
	}
	return strings.TrimSpace(result.Content[0].Text), nil
}

func (s *AgentService) testOpenAICompatible(baseURL, apiKey string, req *dto.TestAgentConnectionRequest) (string, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 || maxTokens > 64 {
		maxTokens = 32
	}

	endpoint := baseURL
	if !strings.HasSuffix(strings.ToLower(endpoint), "/chat/completions") {
		endpoint += "/chat/completions"
	}

	requestBody := map[string]any{
		"model":       req.ModelName,
		"max_tokens":  maxTokens,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with PONG only."},
		},
	}

	body, _ := json.Marshal(requestBody)
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求模型服务失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("模型服务返回错误 %d: %s", resp.StatusCode, truncateOutput(string(respBody), 300))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析模型响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型服务返回空内容")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func truncateOutput(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
