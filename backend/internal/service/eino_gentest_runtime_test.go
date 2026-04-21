package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"

	"go.uber.org/zap"
)

func TestEinoGenTestRuntimeGenerate_WithOpenAITools(t *testing.T) {
	root := t.TempDir()
	config.Global = &config.AppConfig{
		Git: config.GitConfig{
			WorkDir: filepath.Join(root, "repos"),
		},
		CLIRuntime: config.CLIRuntimeConfig{
			WorkspaceRoot:  filepath.Join(root, "runtime"),
			RepoDirName:    "repo",
			ControlDirName: ".autotestflow",
			InputFileName:  "input.json",
			PromptFileName: "prompt.md",
			ResultFileName: "result.json",
			LogFileName:    "runtime.log",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&map[string]any{})

		script := "import { test, expect } from '@playwright/test'\n\ntest('issue-42', async () => {\n  expect(true).toBeTruthy()\n})\n"
		doc := "# 测试用例\n\n## 测试目的\n验证登录成功后跳转首页。\n"
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "已完成仓库写入并准备提交结果。",
						"tool_calls": []map[string]any{
							{
								"id":   "call_write_script",
								"type": "function",
								"function": map[string]any{
									"name":      "Write",
									"arguments": mustJSONString(t, map[string]any{"path": "tests/generated/issue-42.spec.ts", "content": script}),
								},
							},
							{
								"id":   "call_write_doc",
								"type": "function",
								"function": map[string]any{
									"name":      "Write",
									"arguments": mustJSONString(t, map[string]any{"path": "docs/generated/issue-42-test-case.md", "content": doc}),
								},
							},
							{
								"id":   "call_submit",
								"type": "function",
								"function": map[string]any{
									"name": "SubmitGenTestResult",
									"arguments": mustJSONString(t, map[string]any{
										"test_cases": []map[string]any{
											{
												"title":            "登录成功_已有有效账号_跳转首页",
												"category":         "main_flow",
												"precondition":     "已存在有效测试账号",
												"steps":            "1. 打开登录页\n2. 输入账号密码\n3. 点击登录按钮",
												"expected":         "跳转首页并显示欢迎信息",
												"self_test_result": "pass",
												"priority":         1,
											},
										},
										"test_script": map[string]any{
											"file_path": "tests/generated/issue-42.spec.ts",
											"language":  "typescript",
										},
										"test_doc": map[string]any{
											"title":     "测试文档 - 登录成功后跳转首页",
											"file_path": "docs/generated/issue-42-test-case.md",
										},
										"self_test": map[string]any{
											"passed":  true,
											"summary": "模型执行阶段自检通过",
											"checks":  []string{"脚本已写入", "文档已写入"},
										},
										"summary": "生成完成",
									}),
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	task := &model.TestTask{
		ID:        7,
		IssueID:   42,
		ProjectID: 9,
		SkillName: "gen-test",
		Project: &model.Project{
			BaseModel: model.BaseModel{ID: 9},
			Name:      "Demo Project",
			GitBranch: "main",
		},
	}
	input := &GenTestInput{
		ProjectID:     9,
		IssueID:       42,
		ProjectName:   "Demo Project",
		IssueTitle:    "登录后跳转异常",
		IssueSeverity: "high",
	}
	agent := &model.Agent{
		Name:          "Mock OpenAI",
		ModelProvider: "openai",
		ModelName:     "mock-gpt",
		APIKeyRef:     "mock-key-12345678901234567890",
		BaseURL:       server.URL,
		MaxTokens:     1024,
		Temperature:   0.1,
	}

	output, err := runtime.Generate(context.Background(), task, input, nil, agent, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if output.Workspace == nil {
		t.Fatalf("expected workspace metadata")
	}
	if !strings.Contains(output.TestScript.FileContent, "expect(true)") {
		t.Fatalf("expected generated script content, got %s", output.TestScript.FileContent)
	}
	if !strings.Contains(output.TestDoc.Content, "测试目的") {
		t.Fatalf("expected generated doc content, got %s", output.TestDoc.Content)
	}

	scriptPath := filepath.Join(output.Workspace.RepoDir, filepath.FromSlash(output.TestScript.FilePath))
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("expected script file in repo: %v", err)
	}
	docPath := filepath.Join(output.Workspace.RepoDir, filepath.FromSlash(output.TestDoc.FilePath))
	if _, err := os.Stat(docPath); err != nil {
		t.Fatalf("expected doc file in repo: %v", err)
	}
}

func TestEinoGenTestRuntimeRunRead_DirectoryReturnsListing(t *testing.T) {
	repoDir := t.TempDir()
	helperDir := filepath.Join(repoDir, "test-cases", "helpers")
	if err := os.MkdirAll(helperDir, 0o755); err != nil {
		t.Fatalf("mkdir helper dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(helperDir, "auth.ts"), []byte("export const login = () => {}\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runRead(repoDir, "test-cases/helpers")
	if err != nil {
		t.Fatalf("runRead directory returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error directory result, got %#v", result)
	}
	if !strings.Contains(result.Content, "directory: test-cases/helpers") || !strings.Contains(result.Content, "auth.ts") {
		t.Fatalf("expected directory listing, got %q", result.Content)
	}
}

func TestEinoGenTestRuntimeRunRead_MissingPathReturnsHint(t *testing.T) {
	repoDir := t.TempDir()

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runRead(repoDir, "docs")
	if err != nil {
		t.Fatalf("runRead missing path returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error missing-path result, got %#v", result)
	}
	if !strings.Contains(result.Content, "path not found: docs") {
		t.Fatalf("expected not-found hint, got %q", result.Content)
	}
}

func TestEinoGenTestRuntimeRunRead_RejectsPathTraversal(t *testing.T) {
	repoDir := t.TempDir()

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	if _, err := runtime.runRead(repoDir, "../secret.txt"); err == nil {
		t.Fatalf("expected path traversal to be rejected")
	}
}

func TestEinoGenTestRuntimeRunGlob_UsesNativeMatcher(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "tests", "generated"), 0o755); err != nil {
		t.Fatalf("mkdir tests dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "tests", "generated", "issue.spec.ts"), []byte("test('ok', () => {})\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "tests", "generated", "issue.txt"), []byte("not a spec\n"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runGlob(repoDir, "**/*.spec.ts")
	if err != nil {
		t.Fatalf("runGlob returned error: %v", err)
	}
	if !strings.Contains(result.Content, "tests/generated/issue.spec.ts") {
		t.Fatalf("expected spec match, got %q", result.Content)
	}
	if strings.Contains(result.Content, "issue.txt") {
		t.Fatalf("did not expect txt match, got %q", result.Content)
	}
}

func TestEinoGenTestRuntimeRunGrep_UsesNativeSearch(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "src", "login.ts"), []byte("const dataTestId = 'login-button'\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runGrep(repoDir, "login-button", "src", 10)
	if err != nil {
		t.Fatalf("runGrep returned error: %v", err)
	}
	if !strings.Contains(result.Content, "src/login.ts:1:") {
		t.Fatalf("expected grep match, got %q", result.Content)
	}
}

func TestCommandToolSpecs_ByOS(t *testing.T) {
	linuxTools := commandToolSpecs("linux")
	if len(linuxTools) != 1 || linuxTools[0].Name != "Bash" {
		t.Fatalf("expected linux to expose only Bash, got %#v", linuxTools)
	}
	windowsTools := commandToolSpecs("windows")
	if len(windowsTools) != 1 || windowsTools[0].Name != "PowerShell" {
		t.Fatalf("expected windows to expose only PowerShell, got %#v", windowsTools)
	}
}

func mustJSONString(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(data)
}
