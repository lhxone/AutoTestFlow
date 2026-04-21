package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
)

func TestResolveRuntimeWorkspaceConfig_UsesAgentOverrides(t *testing.T) {
	config.Global = &config.AppConfig{
		Git: config.GitConfig{
			WorkDir: filepath.Join(t.TempDir(), "repos"),
		},
		CLIRuntime: config.CLIRuntimeConfig{
			WorkspaceRoot:     filepath.Join(t.TempDir(), "runtime"),
			RepoDirName:       "repo",
			ControlDirName:    ".autotestflow",
			InputFileName:     "input.json",
			PromptFileName:    "prompt.md",
			ResultFileName:    "result.json",
			LogFileName:       "runtime.log",
			PreserveWorkspace: true,
		},
	}

	agentCfg := map[string]any{
		"runtime_type": "eino",
		"cli_runtime": map[string]any{
			"workspace_root":     filepath.Join(t.TempDir(), "agent-runtime"),
			"repo_dir_name":      "repo-agent",
			"control_dir_name":   ".agentflow",
			"input_file_name":    "agent-input.json",
			"prompt_file_name":   "agent-prompt.md",
			"result_file_name":   "agent-result.json",
			"log_file_name":      "agent-runtime.log",
			"preserve_workspace": false,
		},
	}
	raw, err := json.Marshal(agentCfg)
	if err != nil {
		t.Fatalf("marshal agent config: %v", err)
	}

	cfg, err := ResolveRuntimeWorkspaceConfig(&model.Agent{ConfigJSON: model.JSON(raw)})
	if err != nil {
		t.Fatalf("ResolveRuntimeWorkspaceConfig error: %v", err)
	}

	if cfg.WorkspaceRoot != agentCfg["cli_runtime"].(map[string]any)["workspace_root"] {
		t.Fatalf("expected overridden workspace root, got %s", cfg.WorkspaceRoot)
	}
	if cfg.RepoDirName != "repo-agent" {
		t.Fatalf("expected repo-agent, got %s", cfg.RepoDirName)
	}
	if cfg.ControlDirName != ".agentflow" {
		t.Fatalf("expected .agentflow, got %s", cfg.ControlDirName)
	}
	if cfg.InputFileName != "agent-input.json" {
		t.Fatalf("expected agent-input.json, got %s", cfg.InputFileName)
	}
	if cfg.PreserveWorkspace {
		t.Fatalf("expected preserve_workspace false")
	}
}

func TestResolveRuntimeWorkspaceConfig_UsesDefaults(t *testing.T) {
	root := t.TempDir()
	config.Global = &config.AppConfig{
		Git: config.GitConfig{WorkDir: filepath.Join(root, "repos")},
	}

	cfg, err := ResolveRuntimeWorkspaceConfig(nil)
	if err != nil {
		t.Fatalf("ResolveRuntimeWorkspaceConfig error: %v", err)
	}

	if cfg.WorkspaceRoot != filepath.Join(root, "repos", "cli-runtime") {
		t.Fatalf("unexpected workspace root: %s", cfg.WorkspaceRoot)
	}
	if cfg.RepoDirName != defaultRuntimeRepoDirName {
		t.Fatalf("unexpected repo dir name: %s", cfg.RepoDirName)
	}
	if cfg.ControlDirName != defaultRuntimeControlDirName {
		t.Fatalf("unexpected control dir name: %s", cfg.ControlDirName)
	}
	if cfg.InputFileName != defaultRuntimeInputFileName {
		t.Fatalf("unexpected input file name: %s", cfg.InputFileName)
	}
	if cfg.PromptFileName != defaultRuntimePromptFileName {
		t.Fatalf("unexpected prompt file name: %s", cfg.PromptFileName)
	}
	if cfg.ResultFileName != defaultRuntimeResultFileName {
		t.Fatalf("unexpected result file name: %s", cfg.ResultFileName)
	}
	if cfg.LogFileName != defaultRuntimeLogFileName {
		t.Fatalf("unexpected log file name: %s", cfg.LogFileName)
	}
}
