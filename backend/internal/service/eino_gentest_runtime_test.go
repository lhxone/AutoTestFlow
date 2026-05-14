package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
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

func TestEinoGenTestRuntimeRunAgentLoop_TextJSONDoesNotBypassSubmit(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	script := "import { test, expect } from '@playwright/test'\n\ntest('ok', async () => { expect(true).toBeTruthy() })\n"
	doc := "# 测试文档\n"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		message := map[string]any{}
		if requestCount == 1 {
			message["content"] = `{"test_cases":[{"title":"bad","steps":"1","expected":"1"}],"test_script":{"file_path":"tests/bypass.spec.ts"},"test_doc":{"file_path":"docs/bypass.md"},"summary":"should not bypass"}`
		} else {
			message["content"] = "调用工具提交。"
			message["tool_calls"] = []map[string]any{
				{
					"id":   "write_script",
					"type": "function",
					"function": map[string]any{
						"name":      "WriteTestScript",
						"arguments": mustJSONString(t, map[string]any{"path": "tests/final.spec.ts", "content": script, "language": "typescript"}),
					},
				},
				{
					"id":   "write_doc",
					"type": "function",
					"function": map[string]any{
						"name":      "WriteTestDoc",
						"arguments": mustJSONString(t, map[string]any{"path": "docs/final.md", "content": doc, "title": "测试文档"}),
					},
				},
				{
					"id":   "submit",
					"type": "function",
					"function": map[string]any{
						"name": "SubmitGenTestResult",
						"arguments": mustJSONString(t, map[string]any{
							"test_cases": []map[string]any{{"title": "ok", "steps": "1. run", "expected": "pass"}},
							"test_script": map[string]any{
								"file_path": "tests/final.spec.ts",
								"language":  "typescript",
							},
							"test_doc": map[string]any{
								"file_path": "docs/final.md",
							},
							"summary": "submitted",
						}),
					},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": message}},
		})
	}))
	defer server.Close()

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	output, err := runtime.runAgentLoop(context.Background(), AgentExecutionConfig{
		Provider:    "openai",
		Model:       "mock",
		APIKey:      "key",
		BaseURL:     server.URL,
		MaxTokens:   1024,
		Temperature: 0.1,
	}, &genTestToolCallContext{
		Task: &model.TestTask{ID: 1, IssueID: 2, Project: &model.Project{Name: "Demo"}},
		Input: &GenTestInput{
			IssueTitle: "Demo issue",
		},
		Workspace: &RuntimeWorkspace{
			RootDir:    repoDir,
			RepoDir:    repoDir,
			ControlDir: controlDir,
			ResultFile: filepath.Join(controlDir, "result.json"),
		},
	}, nil)
	if err != nil {
		t.Fatalf("runAgentLoop returned error: %v", err)
	}
	if requestCount < 2 {
		t.Fatalf("expected text JSON response not to finish loop, requests=%d", requestCount)
	}
	if output.TestScript.FilePath != "tests/final.spec.ts" {
		t.Fatalf("expected submitted tool output, got %#v", output.TestScript)
	}
}

func TestEinoGenTestRuntimeRunRead_DirectoryReturnsListing(t *testing.T) {
	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	helperDir := filepath.Join(repoDir, "test-cases", "helpers")
	if err := os.MkdirAll(helperDir, 0o755); err != nil {
		t.Fatalf("mkdir helper dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(helperDir, "auth.ts"), []byte("export const login = () => {}\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runRead(&RuntimeWorkspace{RootDir: rootDir, RepoDir: repoDir}, "test-cases/helpers")
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

func TestEinoGenTestRuntimeWriteTestAssets_UpdateDraft(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	runtime := NewEinoGenTestRuntime(zap.NewNop())
	toolCtx := &genTestToolCallContext{
		Input: &GenTestInput{IssueTitle: "登录异常"},
		Workspace: &RuntimeWorkspace{
			RepoDir:    repoDir,
			ControlDir: controlDir,
			ResultFile: filepath.Join(controlDir, "result.json"),
		},
	}
	script := "import { test, expect } from '@playwright/test'\n\ntest('ok', async () => { expect(true).toBeTruthy() })\n"
	if _, err := runtime.executeTool(context.Background(), toolCtx, runtimeToolCall{
		Name:      "WriteTestScript",
		Arguments: map[string]any{"path": "tests/generated.spec.ts", "content": script},
	}); err != nil {
		t.Fatalf("WriteTestScript error: %v", err)
	}
	if _, err := runtime.executeTool(context.Background(), toolCtx, runtimeToolCall{
		Name:      "WriteTestDoc",
		Arguments: map[string]any{"path": "docs/generated.md", "title": "生成文档", "content": "# doc\n"},
	}); err != nil {
		t.Fatalf("WriteTestDoc error: %v", err)
	}
	draft, err := readGenTestDraft(toolCtx.Workspace)
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if draft.TestScript.FilePath != "tests/generated.spec.ts" || !strings.Contains(draft.TestScript.FileContent, "expect(true)") {
		t.Fatalf("unexpected script draft: %#v", draft.TestScript)
	}
	if draft.TestDoc.FilePath != "docs/generated.md" || draft.TestDoc.Title != "生成文档" {
		t.Fatalf("unexpected doc draft: %#v", draft.TestDoc)
	}
}

func TestWriteTestScriptRejectsSpecWithoutTestDeclaration(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	runtime := NewEinoGenTestRuntime(zap.NewNop())
	toolCtx := &genTestToolCallContext{
		Workspace: &RuntimeWorkspace{
			RepoDir:    repoDir,
			ControlDir: controlDir,
			ResultFile: filepath.Join(controlDir, "result.json"),
		},
	}

	_, err := runtime.executeTool(context.Background(), toolCtx, runtimeToolCall{
		Name: "WriteTestScript",
		Arguments: map[string]any{
			"path":    "tests/generated.spec.ts",
			"content": "BASE_URL=http://example.test\n",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "缺少可执行的 Playwright 测试声明") {
		t.Fatalf("expected missing test declaration error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoDir, "tests", "generated.spec.ts")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid spec should not be written, stat err: %v", statErr)
	}
}

func TestBuildSubmittedGenTestOutput_MergesDraftAndRejectsTraversal(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	workspace := &RuntimeWorkspace{RepoDir: repoDir, ControlDir: controlDir, ResultFile: filepath.Join(controlDir, "result.json")}
	script := "import { test, expect } from '@playwright/test'\n\ntest('ok', async () => { expect(true).toBeTruthy() })\n"
	if err := writeRepoFile(repoDir, "tests/from-draft.spec.ts", script); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := writeRepoFile(repoDir, "docs/from-draft.md", "# doc\n"); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	if err := writeGenTestDraft(workspace, &GenTestOutput{
		TestScript: GenTestScript{FilePath: "tests/from-draft.spec.ts", FileContent: script, Language: "typescript"},
		TestDoc:    GenTestDoc{Title: "草稿文档", FilePath: "docs/from-draft.md", Content: "# doc\n"},
		Summary:    "draft",
	}); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	runtime := NewEinoGenTestRuntime(zap.NewNop())
	output, err := runtime.buildSubmittedGenTestOutput(&genTestToolCallContext{Workspace: workspace}, map[string]any{
		"test_cases": []map[string]any{{"title": "ok", "steps": "1. run", "expected": "pass"}},
		"test_script": map[string]any{
			"file_path": "tests/from-draft.spec.ts",
		},
		"test_doc": map[string]any{
			"file_path": "docs/from-draft.md",
		},
		"summary": "submitted",
	})
	if err != nil {
		t.Fatalf("buildSubmittedGenTestOutput error: %v", err)
	}
	if !strings.Contains(output.TestScript.FileContent, "expect(true)") || output.TestDoc.Content == "" {
		t.Fatalf("expected content hydrated from draft/repo, got %#v %#v", output.TestScript, output.TestDoc)
	}
	if _, err := runtime.buildSubmittedGenTestOutput(&genTestToolCallContext{Workspace: workspace}, map[string]any{
		"test_cases": []map[string]any{{"title": "bad", "steps": "1", "expected": "bad"}},
		"test_script": map[string]any{
			"file_path": "../outside.spec.ts",
		},
		"test_doc": map[string]any{
			"file_path": "docs/from-draft.md",
		},
		"summary": "bad",
	}); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
}

func TestEinoGenTestRuntimeRunRead_MissingPathReturnsHint(t *testing.T) {
	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runRead(&RuntimeWorkspace{RootDir: rootDir, RepoDir: repoDir}, "docs")
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
	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	if _, err := runtime.runRead(&RuntimeWorkspace{RootDir: rootDir, RepoDir: repoDir}, "../secret.txt"); err == nil {
		t.Fatalf("expected path traversal to be rejected")
	}
}

func TestEinoGenTestRuntimeRunRead_AllowsAbsolutePathWithinWorkspace(t *testing.T) {
	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	controlDir := filepath.Join(rootDir, ".autotestflow")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	promptFile := filepath.Join(controlDir, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("# prompt\n"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runRead(&RuntimeWorkspace{RootDir: rootDir, RepoDir: repoDir}, promptFile)
	if err != nil {
		t.Fatalf("runRead absolute path returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error absolute-path result, got %#v", result)
	}
	if !strings.Contains(result.Content, "# prompt") {
		t.Fatalf("expected prompt content, got %q", result.Content)
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

func TestEinoGenTestRuntimeRunCommand_EmptyOutputReturnsMessage(t *testing.T) {
	repoDir := t.TempDir()
	program := "sh"
	args := []string{"-c", "true"}
	if stdruntime.GOOS == "windows" {
		program = "cmd"
		args = []string{"/c", "exit", "0"}
	}

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	result, err := runtime.runCommand(context.Background(), &genTestToolCallContext{
		Workspace: &RuntimeWorkspace{RepoDir: repoDir},
	}, program, args)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error command result, got %#v", result)
	}
	if !strings.Contains(result.Content, "completed successfully with no output") {
		t.Fatalf("expected explicit empty-output success message, got %q", result.Content)
	}
}

func TestDetectGeneratedTestCommand_ByScriptLanguage(t *testing.T) {
	repoDir := t.TempDir()
	cases := []struct {
		path    string
		content string
		lang    string
		want    string
	}{
		{"tests/issue.spec.ts", "test('ok', () => {})\n", "typescript", "npx playwright test"},
		{"tests/issue.spec.js", "test('ok', () => {})\n", "javascript", "npx playwright test"},
		{"tests/test_issue.py", "def test_ok():\n    assert True\n", "python", "python -m pytest"},
	}
	for _, tc := range cases {
		if err := writeRepoFile(repoDir, tc.path, tc.content); err != nil {
			t.Fatalf("write %s: %v", tc.path, err)
		}
		cmdSpec, err := detectGeneratedTestCommand(repoDir, GenTestScript{FilePath: tc.path, Language: tc.lang})
		if err != nil {
			t.Fatalf("detectGeneratedTestCommand(%s) error: %v", tc.path, err)
		}
		if !strings.Contains(cmdSpec.Display, tc.want) {
			t.Fatalf("expected command %q to contain %q", cmdSpec.Display, tc.want)
		}
	}
}

func TestRunGeneratedTestCommand_RecordsSuccessAndFailure(t *testing.T) {
	repoDir := t.TempDir()
	successProgram := "sh"
	successArgs := []string{"-c", "echo ok"}
	failProgram := "sh"
	failArgs := []string{"-c", "echo bad; exit 3"}
	if stdruntime.GOOS == "windows" {
		successProgram = "cmd"
		successArgs = []string{"/c", "echo ok"}
		failProgram = "cmd"
		failArgs = []string{"/c", "echo bad & exit /b 3"}
	}
	success := runGeneratedTestCommand(context.Background(), repoDir, generatedTestCommand{
		Display: "success",
		Program: successProgram,
		Args:    successArgs,
	})
	if !success.Passed || !strings.Contains(success.Output, "ok") {
		t.Fatalf("expected success attempt, got %#v", success)
	}
	failure := runGeneratedTestCommand(context.Background(), repoDir, generatedTestCommand{
		Display: "failure",
		Program: failProgram,
		Args:    failArgs,
	})
	if failure.Passed || failure.ExitCode != 3 || !strings.Contains(failure.Output, "bad") {
		t.Fatalf("expected failure attempt with exit code/output, got %#v", failure)
	}
}

func TestCompactRuntimeHistory_ReplacesOldToolHistoryWithSummary(t *testing.T) {
	history := []runtimeMessage{
		{Role: "system", Text: "system prompt"},
		{Role: "user", Text: "initial prompt"},
	}
	for i := 0; i < 10; i++ {
		history = append(history,
			runtimeMessage{
				Role: "assistant",
				Text: "reading repository",
				ToolCalls: []runtimeToolCall{
					{ID: fmt.Sprintf("call_%d", i), Name: "Read", Arguments: map[string]any{"path": fmt.Sprintf("src/file-%d.ts", i)}},
				},
			},
			runtimeMessage{
				Role:       "tool",
				ToolCallID: fmt.Sprintf("call_%d", i),
				ToolResult: strings.Repeat("large output ", 80),
			},
		)
	}

	compacted, changed, compactedCount := compactRuntimeHistory(context.Background(), history, 2_500, 6)
	if !changed {
		t.Fatalf("expected history to be compacted")
	}
	if compactedCount == 0 {
		t.Fatalf("expected compacted message count")
	}
	if compacted[0].Role != "system" || compacted[1].Role != "user" {
		t.Fatalf("expected system and initial user prompt to be preserved, got %#v", compacted[:2])
	}
	if compacted[2].Role == "tool" {
		t.Fatalf("compacted history must not start recent tail with an orphan tool result")
	}
	if estimateRuntimeHistoryBytes(compacted) >= estimateRuntimeHistoryBytes(history) {
		t.Fatalf("expected compacted history to be smaller")
	}
	foundEinoPlaceholder := false
	foundSummary := false
	for _, msg := range compacted {
		if strings.Contains(msg.ToolResult, "Eino reduction") {
			foundEinoPlaceholder = true
		}
		if msg.Role == "user" && strings.HasPrefix(strings.TrimSpace(msg.Text), genTestContextSummaryPrefix) {
			foundSummary = true
		}
	}
	if !foundEinoPlaceholder && !foundSummary {
		t.Fatalf("expected Eino reduction placeholder or structural summary in compacted history")
	}
}

func TestCompactRuntimeHistory_TrimsOversizedInitialPrompt(t *testing.T) {
	history := []runtimeMessage{
		{Role: "system", Text: "system prompt"},
		{Role: "user", Text: "# AutoTestFlow Eino Runtime\n\n## 输入上下文 JSON\n" + strings.Repeat("large input\n", 2000)},
	}
	compacted, changed, compactedCount := compactRuntimeHistory(context.Background(), history, 8_000, 6)
	if !changed {
		t.Fatalf("expected oversized initial prompt to be compacted")
	}
	if compactedCount == 0 {
		t.Fatalf("expected compacted count")
	}
	if estimateRuntimeHistoryBytes(compacted) >= estimateRuntimeHistoryBytes(history) {
		t.Fatalf("expected compacted history to be smaller")
	}
	if !strings.Contains(compacted[1].Text, "该段已由运行时裁剪") && !strings.Contains(compacted[1].Text, "truncated") {
		t.Fatalf("expected trimmed marker in prompt, got %q", compacted[1].Text)
	}
}

func TestNormalizeOpenAICompatibleBaseURL(t *testing.T) {
	cases := map[string]string{
		"http://example.test":                                   "http://example.test/v1",
		"http://example.test/v1":                                "http://example.test/v1",
		"https://open.bigmodel.cn/api/paas/v4":                  "https://open.bigmodel.cn/api/paas/v4",
		"http://example.test/custom/chat/completions":           "http://example.test/custom",
		"http://example.test/v1/chat/completions":               "http://example.test/v1",
		"https://open.bigmodel.cn/api/paas/v4/":                 "https://open.bigmodel.cn/api/paas/v4",
		"https://open.bigmodel.cn/api/paas/v4/chat/completions": "https://open.bigmodel.cn/api/paas/v4",
	}
	for input, want := range cases {
		if got := normalizeOpenAICompatibleBaseURL(input); got != want {
			t.Fatalf("normalizeOpenAICompatibleBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildEinoToolInfos_DeduplicatesNames(t *testing.T) {
	tools, err := buildEinoToolInfos([]genTestToolSpec{
		{Name: "Read", Description: "first", Schema: map[string]any{"type": "object"}},
		{Name: "Read", Description: "second", Schema: map[string]any{"type": "object"}},
		{Name: "Write", Description: "third", Schema: map[string]any{"type": "object"}},
	})
	if err != nil {
		t.Fatalf("buildEinoToolInfos returned error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 unique tools, got %d", len(tools))
	}
	if tools[0].Name != "Read" || tools[1].Name != "Write" {
		t.Fatalf("unexpected tool order: %#v", tools)
	}
}

func TestResolveGenTestADKConfig_RuntimeTypePriority(t *testing.T) {
	agentRaw, _ := json.Marshal(map[string]any{
		"runtime_type": "adk",
		"adk": map[string]any{
			"max_iterations":       33,
			"emit_internal_events": false,
		},
	})
	workflowRaw, _ := json.Marshal(map[string]any{
		"runtime_type": "eino",
		"adk": map[string]any{
			"max_iterations": 44,
		},
	})

	cfg := resolveGenTestADKConfig(&model.Skill{ConfigJSON: model.JSON(workflowRaw)}, &model.Agent{ConfigJSON: model.JSON(agentRaw)})
	if cfg.Enabled {
		t.Fatalf("workflow runtime_type should override agent runtime_type")
	}
	if cfg.MaxIterations != 44 {
		t.Fatalf("expected workflow ADK max_iterations override, got %d", cfg.MaxIterations)
	}
	if cfg.EmitInternalEvents {
		t.Fatalf("expected agent emit_internal_events override to remain false")
	}

	cfg = resolveGenTestADKConfig(nil, &model.Agent{ConfigJSON: model.JSON(agentRaw)})
	if !cfg.Enabled {
		t.Fatalf("expected agent runtime_type=adk to enable ADK")
	}
	if cfg.MaxIterations != 33 {
		t.Fatalf("expected agent max_iterations, got %d", cfg.MaxIterations)
	}

	cfg = resolveGenTestADKConfig(nil, nil)
	if cfg.Enabled {
		t.Fatalf("expected default runtime to keep ADK disabled")
	}
}

func TestBuildADKTools_AppliesRoleAllowListAndDeduplicates(t *testing.T) {
	runtime := NewEinoGenTestRuntime(zap.NewNop())
	tools, err := runtime.buildADKTools(nil, []genTestToolSpec{
		{Name: "Read", Description: "read", Schema: map[string]any{"type": "object"}},
		{Name: "Read", Description: "duplicate", Schema: map[string]any{"type": "object"}},
		{Name: "Write", Description: "write file", Schema: map[string]any{"type": "object"}},
		{Name: "WriteTestScript", Description: "write", Schema: map[string]any{"type": "object"}},
		{Name: "SubmitGenTestResult", Description: "submit", Schema: map[string]any{"type": "object"}},
	}, genTestWriterToolSet(), &sync.Mutex{}, new(*GenTestOutput))
	if err != nil {
		t.Fatalf("buildADKTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatalf("tool info error: %v", err)
		}
		names = append(names, info.Name)
	}
	got := strings.Join(names, ",")
	if got != "Read,Write,WriteTestScript" {
		t.Fatalf("unexpected tool list: %s", got)
	}
}

func TestADKWriterToolSetAllowsGenericWriteButNotSubmit(t *testing.T) {
	allow := genTestWriterToolSet()
	for _, name := range []string{"Write", "WriteTestScript", "WriteTestDoc", "Edit"} {
		if _, ok := allow[name]; !ok {
			t.Fatalf("expected writer tool set to allow %s", name)
		}
	}
	if _, ok := allow["SubmitGenTestResult"]; ok {
		t.Fatalf("writer sub-agent must not be allowed to submit final result")
	}
}

func TestADKToolCommandFailureReturnsObservation(t *testing.T) {
	repoDir := t.TempDir()
	runtimeSvc := NewEinoGenTestRuntime(zap.NewNop())
	toolCtx := &genTestToolCallContext{
		Workspace: &RuntimeWorkspace{RepoDir: repoDir},
	}

	toolName := "Bash"
	command := "echo missing dependency; exit 1"
	if stdruntime.GOOS == "windows" {
		toolName = "PowerShell"
		command = "Write-Output 'missing dependency'; exit 1"
	}

	adkTool := &genTestADKTool{
		spec: genTestToolSpec{
			Name:        toolName,
			Description: "run command",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
		runtime: runtimeSvc,
		toolCtx: toolCtx,
	}

	result, err := adkTool.InvokableRun(context.Background(), mustJSONString(t, map[string]any{"command": command}))
	if err != nil {
		t.Fatalf("expected command failure to be returned as tool observation, got error: %v", err)
	}
	if !strings.Contains(result, "missing dependency") || !strings.Contains(result, "exit status") {
		t.Fatalf("unexpected command failure observation: %s", result)
	}
}

func TestADKToolValidationFailureReturnsObservation(t *testing.T) {
	repoDir := t.TempDir()
	runtimeSvc := NewEinoGenTestRuntime(zap.NewNop())
	adkTool := &genTestADKTool{
		spec: genTestToolSpec{
			Name:        "Read",
			Description: "read",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		runtime: runtimeSvc,
		toolCtx: &genTestToolCallContext{
			Workspace: &RuntimeWorkspace{RepoDir: repoDir},
		},
	}

	result, err := adkTool.InvokableRun(context.Background(), mustJSONString(t, map[string]any{"pattern": "**/*.ts"}))
	if err != nil {
		t.Fatalf("expected validation failure to be returned as tool observation, got error: %v", err)
	}
	if !strings.Contains(result, "tool execution failed") || !strings.Contains(result, "缺少参数: path") {
		t.Fatalf("unexpected validation failure observation: %s", result)
	}
}

func TestExecuteToolReadAcceptsPatternAliasForPlainPath(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "playwright.config.ts"), []byte("export default {};"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runtimeSvc := NewEinoGenTestRuntime(zap.NewNop())

	result, err := runtimeSvc.executeTool(context.Background(), &genTestToolCallContext{
		Workspace: &RuntimeWorkspace{RepoDir: repoDir},
	}, runtimeToolCall{Name: "Read", Arguments: map[string]any{"pattern": "playwright.config.ts"}})
	if err != nil {
		t.Fatalf("expected Read pattern alias to succeed, got error: %v", err)
	}
	if result == nil || !strings.Contains(result.Content, "export default") {
		t.Fatalf("unexpected read result: %#v", result)
	}
}

func TestRecoverADKDraftOutput_FromUnsubmittedDraft(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(filepath.Join(repoDir, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "tests", "fallback.spec.ts"), []byte("import { test, expect } from '@playwright/test'\n\ntest('fallback', async () => { expect(true).toBeTruthy() })\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "docs", "fallback.md"), []byte("# fallback doc\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	workspace := &RuntimeWorkspace{
		RepoDir:    repoDir,
		ControlDir: controlDir,
		ResultFile: filepath.Join(controlDir, "result.json"),
	}
	if err := writeGenTestDraft(workspace, &GenTestOutput{
		TestScript: GenTestScript{FilePath: "tests/fallback.spec.ts", Language: "typescript"},
		TestDoc:    GenTestDoc{Title: "fallback", FilePath: "docs/fallback.md"},
	}); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	output, recovered := NewEinoGenTestRuntime(zap.NewNop()).recoverADKDraftOutput(context.Background(), &genTestToolCallContext{
		Task:      &model.TestTask{ID: 99},
		Workspace: workspace,
	}, fmt.Errorf("Eino ADK 运行时未提交结构化结果"))
	if !recovered {
		t.Fatalf("expected draft to be recovered")
	}
	if !strings.Contains(output.TestScript.FileContent, "playwright") {
		t.Fatalf("expected script content to be hydrated, got %#v", output.TestScript)
	}
	if !strings.Contains(output.TestDoc.Content, "fallback doc") {
		t.Fatalf("expected doc content to be hydrated, got %#v", output.TestDoc)
	}
	if output.SelfTest == nil || output.SelfTest.Passed {
		t.Fatalf("expected recovered output to carry a warning self-test report: %#v", output.SelfTest)
	}
}

func TestRecoverADKDraftOutput_FromDeadlineContext(t *testing.T) {
	repoDir := t.TempDir()
	controlDir := filepath.Join(repoDir, ".autotestflow")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	workspace := &RuntimeWorkspace{
		RepoDir:    repoDir,
		ControlDir: controlDir,
		ResultFile: filepath.Join(controlDir, "result.json"),
	}
	if err := writeGenTestDraft(workspace, &GenTestOutput{
		TestScript: GenTestScript{FilePath: "tests/from-draft.spec.ts", FileContent: "test('draft', async () => {})", Language: "typescript"},
	}); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	output, recovered := NewEinoGenTestRuntime(zap.NewNop()).recoverADKDraftOutput(ctx, &genTestToolCallContext{
		Task:      &model.TestTask{ID: 100},
		Workspace: workspace,
	}, context.Canceled)
	if !recovered {
		t.Fatalf("expected canceled context draft to be recovered")
	}
	if output.TestScript.FilePath != "tests/from-draft.spec.ts" {
		t.Fatalf("unexpected recovered script path: %#v", output.TestScript)
	}
	if output.SelfTest == nil || !strings.Contains(strings.Join(output.SelfTest.Checks, "\n"), context.Canceled.Error()) {
		t.Fatalf("expected deadline/cancel reason in checks, got %#v", output.SelfTest)
	}
}

func TestEinoGenTestRuntimeGenerate_WithADKDeepAgent(t *testing.T) {
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

	mainCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		toolNames := requestToolNames(body)

		message := map[string]any{"content": ""}
		switch {
		case containsString(toolNames, "task"):
			mainCalls++
			if mainCalls == 1 {
				message["content"] = "先委派子代理探索仓库。"
				message["tool_calls"] = []map[string]any{{
					"id":   "call_task",
					"type": "function",
					"function": map[string]any{
						"name":      "task",
						"arguments": mustJSONString(t, map[string]any{"subagent_type": genTestADKExplorerAgentName, "description": "探索测试结构"}),
					},
				}}
			} else {
				script := "import { test, expect } from '@playwright/test'\n\ntest('adk issue', async () => { expect(true).toBeTruthy() })\n"
				doc := "# ADK 测试文档\n\n## 测试目的\n验证 ADK 生成路径。\n"
				message["content"] = "探索完成，写入并提交。"
				message["tool_calls"] = []map[string]any{
					{
						"id":   "write_script",
						"type": "function",
						"function": map[string]any{
							"name":      "WriteTestScript",
							"arguments": mustJSONString(t, map[string]any{"path": "tests/adk.spec.ts", "content": script, "language": "typescript"}),
						},
					},
					{
						"id":   "write_doc",
						"type": "function",
						"function": map[string]any{
							"name":      "WriteTestDoc",
							"arguments": mustJSONString(t, map[string]any{"path": "docs/adk.md", "content": doc, "title": "ADK 测试文档"}),
						},
					},
					{
						"id":   "submit",
						"type": "function",
						"function": map[string]any{
							"name": "SubmitGenTestResult",
							"arguments": mustJSONString(t, map[string]any{
								"test_cases": []map[string]any{{"title": "adk", "steps": "1. run", "expected": "pass"}},
								"test_script": map[string]any{
									"file_path": "tests/adk.spec.ts",
									"language":  "typescript",
								},
								"test_doc": map[string]any{
									"title":     "ADK 测试文档",
									"file_path": "docs/adk.md",
								},
								"summary": "adk submitted",
							}),
						},
					},
				}
			}
		default:
			message["content"] = "repo_explorer: 已确认测试结构。"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": message}}})
	}))
	defer server.Close()

	runtime := NewEinoGenTestRuntime(zap.NewNop())
	workflowCfg, _ := json.Marshal(map[string]any{
		"runtime_type": "adk",
		"adk": map[string]any{
			"max_iterations":       10,
			"emit_internal_events": true,
		},
	})
	task := &model.TestTask{
		ID:        17,
		IssueID:   42,
		ProjectID: 9,
		SkillName: "gen-test",
		Project: &model.Project{
			BaseModel: model.BaseModel{ID: 9},
			Name:      "Demo Project",
			GitBranch: "main",
		},
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
	output, err := runtime.Generate(context.Background(), task, &GenTestInput{
		ProjectID:     9,
		IssueID:       42,
		ProjectName:   "Demo Project",
		IssueTitle:    "ADK 生成测试",
		IssueSeverity: "high",
	}, &model.Skill{Name: "gen-test", ConfigJSON: model.JSON(workflowCfg)}, agent, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if output.TestScript.FilePath != "tests/adk.spec.ts" {
		t.Fatalf("expected ADK submitted script, got %#v", output.TestScript)
	}
	if mainCalls < 2 {
		t.Fatalf("expected main agent to call task then submit, mainCalls=%d", mainCalls)
	}
}

func requestToolNames(body map[string]any) []string {
	rawTools, _ := body["tools"].([]any)
	names := make([]string, 0, len(rawTools))
	for _, raw := range rawTools {
		toolMap, _ := raw.(map[string]any)
		fn, _ := toolMap["function"].(map[string]any)
		name, _ := fn["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustJSONString(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(data)
}
