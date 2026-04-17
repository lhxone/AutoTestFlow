package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"

	"go.uber.org/zap"
)

func TestResolveCLIRuntimeConfig_UsesAgentOverrides(t *testing.T) {
	config.Global = &config.AppConfig{
		Git: config.GitConfig{
			WorkDir: filepath.Join(t.TempDir(), "repos"),
		},
		CLIRuntime: config.CLIRuntimeConfig{
			Command:           "codex",
			Args:              []string{"run", "{{prompt_file}}"},
			Timeout:           "15m",
			WorkspaceRoot:     filepath.Join(t.TempDir(), "runtime"),
			RepoDirName:       "repo",
			ControlDirName:    ".autotestflow",
			InputFileName:     "input.json",
			PromptFileName:    "prompt.md",
			ResultFileName:    "result.json",
			LogFileName:       "cli.log",
			PreserveWorkspace: true,
			Env: map[string]string{
				"CLI_MODE": "global",
			},
		},
	}

	agentCfg := map[string]any{
		"runtime_type": "cli",
		"cli_runtime": map[string]any{
			"command":            "claude",
			"args":               []string{"--print", "{{result_file}}"},
			"timeout":            "5m",
			"workspace_root":     filepath.Join(t.TempDir(), "agent-runtime"),
			"preserve_workspace": false,
			"env": map[string]string{
				"CLI_MODE": "agent",
			},
		},
	}
	raw, err := json.Marshal(agentCfg)
	if err != nil {
		t.Fatalf("marshal agent config: %v", err)
	}

	cfg, err := ResolveCLIRuntimeConfig(&model.Agent{ConfigJSON: model.JSON(raw)})
	if err != nil {
		t.Fatalf("ResolveCLIRuntimeConfig error: %v", err)
	}

	if cfg.Command != "claude" {
		t.Fatalf("expected command claude, got %s", cfg.Command)
	}
	if len(cfg.Args) != 2 || cfg.Args[0] != "--print" {
		t.Fatalf("expected overridden args, got %#v", cfg.Args)
	}
	if cfg.Timeout.String() != "5m0s" {
		t.Fatalf("expected timeout 5m0s, got %s", cfg.Timeout)
	}
	if cfg.PreserveWorkspace {
		t.Fatalf("expected preserve_workspace false")
	}
	if got := cfg.Env["CLI_MODE"]; got != "agent" {
		t.Fatalf("expected CLI_MODE from agent override, got %s", got)
	}
}

func TestCLIRuntimeGenerate_WritesAndReadsArtifacts(t *testing.T) {
	root := t.TempDir()
	config.Global = &config.AppConfig{
		Git: config.GitConfig{
			WorkDir: filepath.Join(root, "repos"),
		},
		CLIRuntime: config.CLIRuntimeConfig{
			Command:           os.Args[0],
			Args:              []string{"-test.run=TestCLIRuntimeHelperProcess", "--", "{{prompt_file}}"},
			Timeout:           "2m",
			WorkspaceRoot:     filepath.Join(root, "runtime"),
			RepoDirName:       "repo",
			ControlDirName:    ".autotestflow",
			InputFileName:     "input.json",
			PromptFileName:    "prompt.md",
			ResultFileName:    "result.json",
			LogFileName:       "cli.log",
			PreserveWorkspace: true,
			Env: map[string]string{
				"GO_WANT_HELPER_PROCESS": "1",
			},
		},
	}

	runtime := NewCLIRuntime(zap.NewNop())
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
		ProjectID:      9,
		IssueID:        42,
		ProjectName:    "Demo Project",
		IssueTitle:     "登录后跳转异常",
		IssueDesc:      "登录成功后未进入首页",
		IssueSeverity:  "high",
		FuncDocPath:    "docs/func.md",
		DesignDocPath:  "docs/design.md",
		DBDocPath:      "docs/db.md",
		TestDocPath:    "docs/tests.md",
		ExtraFilesPath: "fixtures",
	}

	output, err := runtime.Generate(context.Background(), task, input, nil, nil, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if output.Workspace == nil {
		t.Fatalf("expected workspace metadata")
	}
	if !strings.Contains(output.TestScript.FileContent, "playwright") {
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

func TestCLIRuntimeBuildPrompt_IncludesGenTestFlowAndChromeMCP(t *testing.T) {
	runtime := NewCLIRuntime(zap.NewNop())
	workspace := &CLIRuntimeWorkspace{
		RepoDir:    `D:\repo`,
		InputFile:  `D:\repo\.autotestflow\input.json`,
		ResultFile: `D:\repo\.autotestflow\result.json`,
	}
	task := &model.TestTask{
		ID:      1,
		IssueID: 1001,
		Project: &model.Project{Name: "Demo"},
	}
	input := &GenTestInput{IssueTitle: "登录异常"}

	prompt := runtime.buildPrompt(workspace, task, input, nil, &model.Agent{Name: "codex"}, &CLIPromptContext{
		MCPCapabilitySummary: "MCP Server chrome 可用工具: navigate, click",
		ChromeMCPServers:     []string{"chrome-devtools"},
	})

	if !strings.Contains(prompt, "必须严格遵循 gen-test 技能流程推进") {
		t.Fatalf("expected gen-test workflow instruction in prompt")
	}
	if !strings.Contains(prompt, "Chrome MCP") {
		t.Fatalf("expected chrome mcp note in prompt")
	}
	if !strings.Contains(prompt, "MCP Server chrome 可用工具") {
		t.Fatalf("expected capability summary in prompt")
	}
}

func TestCLIRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	repoDir := os.Getenv("ATF_REPO_DIR")
	resultFile := os.Getenv("ATF_RESULT_FILE")
	if repoDir == "" || resultFile == "" {
		os.Exit(2)
	}

	scriptRel := "tests/generated/issue-42.spec.ts"
	docRel := "docs/generated/issue-42-test-case.md"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, filepath.FromSlash(scriptRel))), 0o755); err != nil {
		os.Exit(3)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, filepath.FromSlash(docRel))), 0o755); err != nil {
		os.Exit(4)
	}

	script := "import { test, expect } from '@playwright/test'\n\ntest('issue-42', async () => {\n  expect(true).toBeTruthy()\n})\n"
	doc := "# 测试用例\n\n## 测试目的\n验证登录成功后跳转首页。\n"
	if err := os.WriteFile(filepath.Join(repoDir, filepath.FromSlash(scriptRel)), []byte(script), 0o644); err != nil {
		os.Exit(5)
	}
	if err := os.WriteFile(filepath.Join(repoDir, filepath.FromSlash(docRel)), []byte(doc), 0o644); err != nil {
		os.Exit(6)
	}

	result := map[string]any{
		"test_cases": []map[string]any{
			{
				"title":            "登录成功_已有有效账号_跳转首页",
				"category":         "main_flow",
				"precondition":     "已存在有效测试账号",
				"steps":            "1. 打开登录页\n2. 输入账号密码\n3. 点击登录按钮",
				"expected":         "系统跳转首页并显示欢迎信息",
				"self_test_result": "pass",
				"priority":         1,
			},
		},
		"test_script": map[string]any{
			"file_path": scriptRel,
			"language":  "typescript",
		},
		"test_doc": map[string]any{
			"title":     "登录跳转测试文档",
			"file_path": docRel,
		},
		"self_test": map[string]any{
			"passed":  true,
			"summary": "CLI 自测通过",
			"checks":  []string{"脚本已生成", "文档已生成"},
		},
		"summary": "已生成登录问题测试资产",
	}

	data, err := json.Marshal(result)
	if err != nil {
		os.Exit(7)
	}
	if err := os.WriteFile(resultFile, data, 0o644); err != nil {
		os.Exit(8)
	}
	os.Exit(0)
}
