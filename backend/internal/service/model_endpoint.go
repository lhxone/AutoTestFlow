package service

import (
	"regexp"
	"strings"
)

var versionSuffixPattern = regexp.MustCompile(`/v[0-9]+$`)

func resolveOpenAICompatibleEndpoint(baseURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		return ""
	}

	lower := strings.ToLower(endpoint)
	if strings.HasSuffix(lower, "/chat/completions") {
		return endpoint
	}
	if versionSuffixPattern.MatchString(lower) {
		return endpoint + "/chat/completions"
	}
	return endpoint + "/v1/chat/completions"
}

func resolveAnthropicMessagesEndpoint(baseURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		return ""
	}

	lower := strings.ToLower(endpoint)
	if strings.HasSuffix(lower, "/messages") {
		return endpoint
	}
	if strings.HasSuffix(lower, "/v1") {
		return endpoint + "/messages"
	}
	return endpoint + "/v1/messages"
}
