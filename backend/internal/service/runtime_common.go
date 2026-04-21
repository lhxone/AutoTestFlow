package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"auto-test-flow/internal/model"
)

const (
	defaultRuntimeRepoDirName    = "repo"
	defaultRuntimeControlDirName = ".autotestflow"
	defaultRuntimeInputFileName  = "input.json"
	defaultRuntimePromptFileName = "prompt.md"
	defaultRuntimeResultFileName = "result.json"
	defaultRuntimeLogFileName    = "cli.log"
)

type RuntimeWorkspace struct {
	RootDir           string `json:"root_dir"`
	RepoDir           string `json:"repo_dir"`
	ControlDir        string `json:"control_dir"`
	InputFile         string `json:"input_file"`
	PromptFile        string `json:"prompt_file"`
	ResultFile        string `json:"result_file"`
	LogFile           string `json:"log_file"`
	SharedNodeModules string `json:"shared_node_modules,omitempty"`
}

type CLIPromptContext struct {
	MCPCapabilitySummary string   `json:"mcp_capability_summary,omitempty"`
	ChromeMCPServers     []string `json:"chrome_mcp_servers,omitempty"`
}

type agentRuntimeSettings struct {
	RuntimeType string                 `json:"runtime_type"`
	CLIRuntime  *agentCLIRuntimeConfig `json:"cli_runtime,omitempty"`
}

type agentCLIRuntimeConfig struct {
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	Timeout           string            `json:"timeout"`
	WorkspaceRoot     string            `json:"workspace_root"`
	RepoDirName       string            `json:"repo_dir_name"`
	ControlDirName    string            `json:"control_dir_name"`
	InputFileName     string            `json:"input_file_name"`
	PromptFileName    string            `json:"prompt_file_name"`
	ResultFileName    string            `json:"result_file_name"`
	LogFileName       string            `json:"log_file_name"`
	PreserveWorkspace *bool             `json:"preserve_workspace"`
	Env               map[string]string `json:"env"`
}

func parseAgentRuntimeSettings(agent *model.Agent) (*agentRuntimeSettings, error) {
	if agent == nil || len(agent.ConfigJSON) == 0 {
		return nil, nil
	}
	var settings agentRuntimeSettings
	if err := json.Unmarshal(agent.ConfigJSON, &settings); err != nil {
		return nil, fmt.Errorf("解析 Agent ConfigJSON 失败: %w", err)
	}
	return &settings, nil
}

func writeRepoFile(repoDir, relativePath, content string) error {
	fullPath := filepath.Join(repoDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("创建产物目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入产物文件失败: %w", err)
	}
	return nil
}

func readRepoFile(repoDir, relativePath string) (string, error) {
	fullPath := filepath.Join(repoDir, filepath.FromSlash(relativePath))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &artifactFileNotFoundError{Path: fullPath, RelativePath: relativePath}
		}
		return "", fmt.Errorf("读取产物文件失败: %w", err)
	}
	return string(data), nil
}

type artifactFileNotFoundError struct {
	Path         string
	RelativePath string
}

func (e *artifactFileNotFoundError) Error() string {
	return fmt.Sprintf("产物文件不存在: %s (相对路径: %s)", e.Path, e.RelativePath)
}

func normalizeRepoRelativePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, ".\\")
	return filepath.ToSlash(path)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func truncateText(text string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return trimmed
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
}

func projectDefaultBranch(project *model.Project) string {
	if project != nil && strings.TrimSpace(project.GitBranch) != "" {
		return strings.TrimSpace(project.GitBranch)
	}
	return "main"
}

func safeAgentName(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	return agent.Name
}

func sanitizePathComponent(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	return replacer.Replace(trimmed)
}

func parseGitURL(raw string) (*url.URL, error) {
	return url.Parse(strings.TrimSpace(raw))
}

func modelGitUserPassword(token string) *url.Userinfo {
	return url.UserPassword("oauth2", token)
}

func scanCROrLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			if b == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

var _ bufio.SplitFunc = scanCROrLF
