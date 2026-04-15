package pkg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExtractJSON 尝试从 AI 响应文本中提取有效 JSON
// 支持三种模式：直接JSON / ```json 代码块 / 首尾花括号提取
func ExtractJSON(text string) (string, error) {
	text = strings.TrimSpace(text)

	// 方式1: 直接就是合法JSON
	if json.Valid([]byte(text)) {
		return text, nil
	}

	// 方式2: 提取 ```json ... ``` 代码块
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n\\s*```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	// 方式3: 提取最外层 { ... }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		candidate := text[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("无法从响应中提取有效JSON，原始长度: %d", len(text))
}

// TruncateString 截断字符串到指定长度
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n[文档已截断，仅展示前" + fmt.Sprintf("%d", maxLen) + "字符]"
}
