package service

import "testing"

func TestResolveOpenAICompatibleEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "openai root",
			baseURL: "https://api.openai.com",
			want:    "https://api.openai.com/v1/chat/completions",
		},
		{
			name:    "explicit v1",
			baseURL: "https://api.openai.com/v1",
			want:    "https://api.openai.com/v1/chat/completions",
		},
		{
			name:    "zhipu v4",
			baseURL: "https://open.bigmodel.cn/api/paas/v4",
			want:    "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:    "already full endpoint",
			baseURL: "https://example.com/custom/chat/completions",
			want:    "https://example.com/custom/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveOpenAICompatibleEndpoint(tt.baseURL); got != tt.want {
				t.Fatalf("resolveOpenAICompatibleEndpoint(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestResolveAnthropicMessagesEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "anthropic root",
			baseURL: "https://api.anthropic.com",
			want:    "https://api.anthropic.com/v1/messages",
		},
		{
			name:    "anthropic v1",
			baseURL: "https://api.anthropic.com/v1",
			want:    "https://api.anthropic.com/v1/messages",
		},
		{
			name:    "already full endpoint",
			baseURL: "https://example.com/custom/messages",
			want:    "https://example.com/custom/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveAnthropicMessagesEndpoint(tt.baseURL); got != tt.want {
				t.Fatalf("resolveAnthropicMessagesEndpoint(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}
