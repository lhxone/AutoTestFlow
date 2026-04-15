package service

import (
	"encoding/json"
	"strconv"
	"strings"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/repository"
)

// LoadCLIRuntimeConfig 从数据库加载 CLI Runtime 配置，不再依赖 config.yaml。
func LoadCLIRuntimeConfig() config.CLIRuntimeConfig {
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
