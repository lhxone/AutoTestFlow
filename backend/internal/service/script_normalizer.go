package service

import (
	"fmt"
	"regexp"
	"strings"

	"auto-test-flow/internal/model"
)

var legacyMockTitlePattern = regexp.MustCompile(`"""测试:\s*(.*?)"""`)

func NormalizeTestScripts(scripts []model.TestScript) []model.TestScript {
	normalized := make([]model.TestScript, 0, len(scripts))
	for _, script := range scripts {
		filePath, content, language := NormalizeScriptFields(script.FilePath, script.FileContent, script.Language)
		script.FilePath = filePath
		script.FileContent = content
		script.Language = language
		normalized = append(normalized, script)
	}
	return normalized
}

func NormalizeScriptFields(filePath, content, language string) (string, string, string) {
	if !isLegacyPytestMock(filePath, content, language) {
		return filePath, content, normalizeScriptLanguage(language, filePath)
	}

	title := extractLegacyMockTitle(content)
	if title == "" {
		title = "Mock Issue"
	}

	return "tests/issue-mock.spec.ts", buildPlaywrightMockScript(title), "typescript"
}

func isLegacyPytestMock(filePath, content, language string) bool {
	lowerPath := strings.ToLower(filePath)
	lowerLanguage := strings.ToLower(language)
	return (strings.HasSuffix(lowerPath, ".py") || lowerLanguage == "python") &&
		strings.Contains(content, "import pytest") &&
		strings.Contains(content, "class TestIssue")
}

func extractLegacyMockTitle(content string) string {
	matches := legacyMockTitlePattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func normalizeScriptLanguage(language, filePath string) string {
	lowerLanguage := strings.ToLower(language)
	if lowerLanguage == "typescript" || lowerLanguage == "javascript" || lowerLanguage == "python" {
		return lowerLanguage
	}

	lowerPath := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lowerPath, ".spec.ts"), strings.HasSuffix(lowerPath, ".ts"):
		return "typescript"
	case strings.HasSuffix(lowerPath, ".js"):
		return "javascript"
	case strings.HasSuffix(lowerPath, ".py"):
		return "python"
	default:
		return "typescript"
	}
}

func buildPlaywrightMockScript(title string) string {
	return fmt.Sprintf(`import { test, expect } from '@playwright/test'

test.describe(%q, () => {
  test('主流程验证', async ({ page }) => {
    // TODO: 替换为真实业务地址和断言
    await page.goto('/')
    await expect(page).toHaveTitle(/AutoTestFlow/)
  })

  test('异常处理验证', async ({ page }) => {
    // TODO: 补充异常场景操作
    await page.goto('/')
    await expect(page.locator('body')).toBeVisible()
  })

  test('边界值验证', async ({ page }) => {
    // TODO: 补充边界值验证逻辑
    await page.goto('/')
    await expect(page.locator('body')).toBeVisible()
  })

  test('回归验证', async ({ page }) => {
    // TODO: 补充回归验证逻辑
    await page.goto('/')
    await expect(page.locator('body')).toBeVisible()
  })
})
`, title)
}
