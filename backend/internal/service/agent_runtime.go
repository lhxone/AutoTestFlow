package service

import (
	"os"
	"strings"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
)

type AgentExecutionConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
}

func ResolveAgentExecutionConfig(agent *model.Agent) AgentExecutionConfig {
	repo := repository.NewSettingRepo()
	global := LoadAIConfig()
	cfg := AgentExecutionConfig{
		Provider:    global.Provider,
		APIKey:      global.APIKey,
		BaseURL:     global.BaseURL,
		Model:       global.Model,
		MaxTokens:   global.MaxTokens,
		Temperature: global.Temperature,
	}

	if agent == nil {
		return normalizeAgentExecutionConfig(cfg)
	}

	if value := strings.TrimSpace(agent.ModelProvider); value != "" {
		cfg.Provider = value
	}
	if value := strings.TrimSpace(agent.ModelName); value != "" {
		cfg.Model = value
	}
	if value := resolveAgentAPIKey(agent.APIKeyRef, repo); value != "" {
		cfg.APIKey = value
	}
	if value := resolveAgentBaseURL(agent, cfg.Provider); value != "" {
		cfg.BaseURL = value
	}
	if agent.MaxTokens > 0 {
		cfg.MaxTokens = agent.MaxTokens
	}
	if agent.Temperature > 0 {
		cfg.Temperature = agent.Temperature
	}

	return normalizeAgentExecutionConfig(cfg)
}

func normalizeAgentExecutionConfig(cfg AgentExecutionConfig) AgentExecutionConfig {
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Model = strings.TrimSpace(cfg.Model)

	if cfg.Provider == "" {
		cfg.Provider = "claude"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURLForProvider(cfg.Provider)
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = 0.3
	}
	return cfg
}

func resolveAgentAPIKey(ref string, repo *repository.SettingRepo) string {
	value := strings.TrimSpace(ref)
	if value == "" {
		return ""
	}
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
		if settingValue := strings.TrimSpace(repo.GetValue(category, value)); settingValue != "" {
			return settingValue
		}
	}
	return value
}

func resolveAgentBaseURL(agent *model.Agent, provider string) string {
	if agent != nil {
		if value := strings.TrimRight(strings.TrimSpace(agent.BaseURL), "/"); value != "" {
			return value
		}
	}
	return defaultBaseURLForProvider(provider)
}

func defaultBaseURLForProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
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

func looksLikeAPIKey(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(value, " ") {
		return false
	}
	return strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "sess-") ||
		strings.HasPrefix(value, "AIza") ||
		len(value) >= 24
}
