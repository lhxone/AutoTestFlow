package service

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
)

type RuntimeWorkspaceConfig struct {
	WorkspaceRoot     string
	RepoDirName       string
	ControlDirName    string
	InputFileName     string
	PromptFileName    string
	ResultFileName    string
	LogFileName       string
	PreserveWorkspace bool
}

func LoadRuntimeWorkspaceConfig() config.CLIRuntimeConfig {
	repo := repository.NewSettingRepo()
	raw := config.CLIRuntimeConfig{}
	if config.Global != nil {
		raw = config.Global.CLIRuntime
	}

	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "command")); value != "" {
		raw.Command = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "timeout")); value != "" {
		raw.Timeout = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "workspace_root")); value != "" {
		raw.WorkspaceRoot = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "repo_dir_name")); value != "" {
		raw.RepoDirName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "control_dir_name")); value != "" {
		raw.ControlDirName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "input_file_name")); value != "" {
		raw.InputFileName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "prompt_file_name")); value != "" {
		raw.PromptFileName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "result_file_name")); value != "" {
		raw.ResultFileName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "log_file_name")); value != "" {
		raw.LogFileName = value
	}
	if value := strings.TrimSpace(repo.GetValue("cli_runtime", "preserve_workspace")); value != "" {
		raw.PreserveWorkspace = firstCLIBool(value, raw.PreserveWorkspace)
	}

	if argsJSON := strings.TrimSpace(repo.GetValue("cli_runtime", "args_json")); argsJSON != "" {
		var args []string
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			raw.Args = args
		}
	}

	if envJSON := strings.TrimSpace(repo.GetValue("cli_runtime", "env_json")); envJSON != "" {
		var env map[string]string
		if err := json.Unmarshal([]byte(envJSON), &env); err == nil {
			raw.Env = env
		}
	}

	return raw
}

func ResolveRuntimeWorkspaceConfig(agent *model.Agent) (RuntimeWorkspaceConfig, error) {
	raw := LoadRuntimeWorkspaceConfig()
	cfg := RuntimeWorkspaceConfig{
		WorkspaceRoot:     strings.TrimSpace(raw.WorkspaceRoot),
		RepoDirName:       strings.TrimSpace(raw.RepoDirName),
		ControlDirName:    strings.TrimSpace(raw.ControlDirName),
		InputFileName:     strings.TrimSpace(raw.InputFileName),
		PromptFileName:    strings.TrimSpace(raw.PromptFileName),
		ResultFileName:    strings.TrimSpace(raw.ResultFileName),
		LogFileName:       strings.TrimSpace(raw.LogFileName),
		PreserveWorkspace: raw.PreserveWorkspace,
	}

	override, err := parseAgentRuntimeSettings(agent)
	if err != nil {
		return RuntimeWorkspaceConfig{}, err
	}
	if override != nil && override.CLIRuntime != nil {
		cli := override.CLIRuntime
		if value := strings.TrimSpace(cli.WorkspaceRoot); value != "" {
			cfg.WorkspaceRoot = value
		}
		if value := strings.TrimSpace(cli.RepoDirName); value != "" {
			cfg.RepoDirName = value
		}
		if value := strings.TrimSpace(cli.ControlDirName); value != "" {
			cfg.ControlDirName = value
		}
		if value := strings.TrimSpace(cli.InputFileName); value != "" {
			cfg.InputFileName = value
		}
		if value := strings.TrimSpace(cli.PromptFileName); value != "" {
			cfg.PromptFileName = value
		}
		if value := strings.TrimSpace(cli.ResultFileName); value != "" {
			cfg.ResultFileName = value
		}
		if value := strings.TrimSpace(cli.LogFileName); value != "" {
			cfg.LogFileName = value
		}
		if cli.PreserveWorkspace != nil {
			cfg.PreserveWorkspace = *cli.PreserveWorkspace
		}
	}

	if cfg.WorkspaceRoot == "" {
		cfg.WorkspaceRoot = filepath.Join(config.Global.Git.WorkDir, "cli-runtime")
	}
	if cfg.RepoDirName == "" {
		cfg.RepoDirName = defaultRuntimeRepoDirName
	}
	if cfg.ControlDirName == "" {
		cfg.ControlDirName = defaultRuntimeControlDirName
	}
	if cfg.InputFileName == "" {
		cfg.InputFileName = defaultRuntimeInputFileName
	}
	if cfg.PromptFileName == "" {
		cfg.PromptFileName = defaultRuntimePromptFileName
	}
	if cfg.ResultFileName == "" {
		cfg.ResultFileName = defaultRuntimeResultFileName
	}
	if cfg.LogFileName == "" {
		cfg.LogFileName = defaultRuntimeLogFileName
	}
	return cfg, nil
}

func firstCLIBool(value string, fallback bool) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	if parsed, err := strconv.ParseBool(trimmed); err == nil {
		return parsed
	}
	if trimmed == "1" {
		return true
	}
	if trimmed == "0" {
		return false
	}
	return fallback
}
