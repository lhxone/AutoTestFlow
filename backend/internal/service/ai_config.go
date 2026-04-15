package service

import (
	"strconv"
	"strings"

	"auto-test-flow/internal/repository"
)

// AIConfig AI配置
type AIConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
}

// LoadAIConfig 从数据库加载AI配置，不再依赖 config.yaml。
func LoadAIConfig() AIConfig {
	repo := repository.NewSettingRepo()
	cfg := AIConfig{
		Provider:    strings.TrimSpace(repo.GetValue("ai", "provider")),
		APIKey:      strings.TrimSpace(repo.GetValue("ai", "api_key")),
		BaseURL:     strings.TrimSpace(repo.GetValue("ai", "base_url")),
		Model:       strings.TrimSpace(repo.GetValue("ai", "model")),
		MaxTokens:   firstAIInt(repo.GetValue("ai", "max_tokens"), 0),
		Temperature: firstAIFloat(repo.GetValue("ai", "temperature"), 0),
	}

	// 设置默认值
	if cfg.Provider == "" {
		cfg.Provider = "claude"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}

	return cfg
}

func firstAIInt(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func firstAIFloat(value string, fallback float64) float64 {
	if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}
