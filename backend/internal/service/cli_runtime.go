package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const (
	defaultCLITimeout        = 20 * time.Minute
	defaultCLIRepoDirName    = "repo"
	defaultCLIControlDirName = ".autotestflow"
	defaultCLIInputFileName  = "input.json"
	defaultCLIPromptFileName = "prompt.md"
	defaultCLIResultFileName = "result.json"
	defaultCLILogFileName    = "cli.log"
)

type CLIRuntime struct {
	logger          *zap.Logger
	settingRepo     *repository.SettingRepo
	interactionRepo *repository.CLIInteractionRepo
	eventHub        *TaskEventHub
}

type CLIRuntimeConfig struct {
	Command           string
	Args              []string
	Env               map[string]string
	Timeout           time.Duration
	WorkspaceRoot     string
	RepoDirName       string
	ControlDirName    string
	InputFileName     string
	PromptFileName    string
	ResultFileName    string
	LogFileName       string
	PreserveWorkspace bool
}

type CLIRuntimeWorkspace struct {
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

func NewCLIRuntime(logger *zap.Logger) *CLIRuntime {
	return &CLIRuntime{
		logger:          logger,
		settingRepo:     repository.NewSettingRepo(),
		interactionRepo: repository.NewCLIInteractionRepo(),
		eventHub:        DefaultTaskEventHub,
	}
}

func (r *CLIRuntime) Generate(
	ctx context.Context,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) (*GenTestOutput, error) {
	if task == nil {
		return nil, fmt.Errorf("测试任务不能为空")
	}
	if task.Project == nil {
		return nil, fmt.Errorf("测试任务缺少项目信息")
	}

	runtimeCfg, err := ResolveCLIRuntimeConfig(agent)
	if err != nil {
		return nil, err
	}

	workspace, err := r.prepareWorkspace(ctx, task.ID, task, runtimeCfg)
	if err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "workspace_prepared", model.TaskStatusRunning,
		fmt.Sprintf("CLI 工作区已准备完成\n  工作区: %s\n  仓库目录: %s", workspace.RootDir, workspace.RepoDir),
		map[string]any{
			"workspace_dir": workspace.RootDir,
			"repo_dir":      workspace.RepoDir,
		})

	if err := r.writeControlFiles(workspace, task, input, workflow, agent, promptCtx); err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "control_files_written", model.TaskStatusRunning,
		fmt.Sprintf("CLI 输入文件和 Prompt 已写入\n  输入文件: %s\n  Prompt: %s\n  结果文件: %s",
			workspace.InputFile, workspace.PromptFile, workspace.ResultFile),
		map[string]any{
			"input_file":  workspace.InputFile,
			"prompt_file": workspace.PromptFile,
			"result_file": workspace.ResultFile,
		})

	if err := r.executeCommand(ctx, runtimeCfg, workspace, task, input, workflow, agent); err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "cli_finished", model.TaskStatusRunning,
		fmt.Sprintf("CLI 命令执行完成，正在读取结果文件: %s", workspace.ResultFile), nil)

	output, err := r.readResult(workspace)
	if err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "result_loaded", model.TaskStatusRunning, "已解析 CLI 结果文件", nil)

	if err := r.syncArtifacts(workspace.RepoDir, task, input, output); err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "artifacts_synced", model.TaskStatusRunning, "测试脚本和测试文档已同步到仓库工作区", map[string]any{
		"script_file": output.TestScript.FilePath,
		"doc_file":    output.TestDoc.FilePath,
	})

	output.Workspace = workspace
	return output, nil
}

func ResolveCLIRuntimeConfig(agent *model.Agent) (CLIRuntimeConfig, error) {
	raw := LoadCLIRuntimeConfig()
	cfg := CLIRuntimeConfig{
		Command:           strings.TrimSpace(raw.Command),
		Args:              append([]string(nil), raw.Args...),
		Env:               cloneStringMap(raw.Env),
		WorkspaceRoot:     strings.TrimSpace(raw.WorkspaceRoot),
		RepoDirName:       strings.TrimSpace(raw.RepoDirName),
		ControlDirName:    strings.TrimSpace(raw.ControlDirName),
		InputFileName:     strings.TrimSpace(raw.InputFileName),
		PromptFileName:    strings.TrimSpace(raw.PromptFileName),
		ResultFileName:    strings.TrimSpace(raw.ResultFileName),
		LogFileName:       strings.TrimSpace(raw.LogFileName),
		PreserveWorkspace: raw.PreserveWorkspace,
	}
	if timeout := strings.TrimSpace(raw.Timeout); timeout != "" {
		parsed, err := time.ParseDuration(timeout)
		if err != nil {
			return CLIRuntimeConfig{}, fmt.Errorf("cli_runtime.timeout 配置无效: %w", err)
		}
		cfg.Timeout = parsed
	}

	override, err := parseAgentRuntimeSettings(agent)
	if err != nil {
		return CLIRuntimeConfig{}, err
	}
	if override != nil {
		if value := strings.TrimSpace(override.RuntimeType); value != "" && !strings.EqualFold(value, "cli") {
			return CLIRuntimeConfig{}, fmt.Errorf("当前仅支持 CLI Runtime，agent runtime_type=%s", value)
		}
		if cli := override.CLIRuntime; cli != nil {
			if value := strings.TrimSpace(cli.Command); value != "" {
				cfg.Command = value
			}
			if len(cli.Args) > 0 {
				cfg.Args = append([]string(nil), cli.Args...)
			}
			if value := strings.TrimSpace(cli.Timeout); value != "" {
				parsed, err := time.ParseDuration(value)
				if err != nil {
					return CLIRuntimeConfig{}, fmt.Errorf("agent cli_runtime.timeout 配置无效: %w", err)
				}
				cfg.Timeout = parsed
			}
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
			for key, value := range cli.Env {
				cfg.Env[key] = value
			}
		}
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultCLITimeout
	}
	if cfg.WorkspaceRoot == "" {
		cfg.WorkspaceRoot = filepath.Join(config.Global.Git.WorkDir, "cli-runtime")
	}
	if cfg.RepoDirName == "" {
		cfg.RepoDirName = defaultCLIRepoDirName
	}
	if cfg.ControlDirName == "" {
		cfg.ControlDirName = defaultCLIControlDirName
	}
	if cfg.InputFileName == "" {
		cfg.InputFileName = defaultCLIInputFileName
	}
	if cfg.PromptFileName == "" {
		cfg.PromptFileName = defaultCLIPromptFileName
	}
	if cfg.ResultFileName == "" {
		cfg.ResultFileName = defaultCLIResultFileName
	}
	if cfg.LogFileName == "" {
		cfg.LogFileName = defaultCLILogFileName
	}
	if cfg.Command == "" {
		return CLIRuntimeConfig{}, fmt.Errorf("CLI Runtime 未配置 command，请在 Agent 管理页面设置 CLI 命令或在系统设置中配置全局 CLI Runtime")
	}

	// 如果是 claude 命令且没有参数，自动添加 --print + stream-json 参数以获取流式输出
	if cfg.Command == "claude" && len(cfg.Args) == 0 {
		cfg.Args = []string{"--print", "--output-format", "stream-json"}
	}

	return cfg, nil
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

func (r *CLIRuntime) prepareWorkspace(ctx context.Context, taskID uint64, task *model.TestTask, runtimeCfg CLIRuntimeConfig) (*CLIRuntimeWorkspace, error) {
	projectDir := filepath.Join(runtimeCfg.WorkspaceRoot, fmt.Sprintf("project_%d", task.ProjectID))
	rootDir := filepath.Join(projectDir, fmt.Sprintf("task_%d", task.ID))
	repoDir := filepath.Join(rootDir, runtimeCfg.RepoDirName)
	controlDir := filepath.Join(rootDir, runtimeCfg.ControlDirName)
	branch := projectDefaultBranch(task.Project)
	sharedRepoDir := filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))

	// 创建共享node_modules目录
	sharedNodeModulesDir := filepath.Join(projectDir, "_shared", "node_modules")

	workspace := &CLIRuntimeWorkspace{
		RootDir:           rootDir,
		RepoDir:           repoDir,
		ControlDir:        controlDir,
		InputFile:         filepath.Join(controlDir, runtimeCfg.InputFileName),
		PromptFile:        filepath.Join(controlDir, runtimeCfg.PromptFileName),
		ResultFile:        filepath.Join(controlDir, runtimeCfg.ResultFileName),
		LogFile:           filepath.Join(controlDir, runtimeCfg.LogFileName),
		SharedNodeModules: sharedNodeModulesDir,
	}

	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 CLI 控制目录失败: %w", err)
	}

	// 准备仓库（git worktree机制）
	if err := r.prepareRepository(ctx, taskID, task.Project, branch, projectDir, repoDir); err != nil {
		return nil, err
	}

	// 准备node_modules（共享缓存机制）
	if err := r.prepareNodeModules(ctx, taskID, task, repoDir, sharedNodeModulesDir, sharedRepoDir); err != nil {
		return nil, err
	}

	return workspace, nil
}

func (r *CLIRuntime) prepareRepository(ctx context.Context, taskID uint64, project *model.Project, branch, projectDir, repoDir string) error {
	if project == nil {
		return fmt.Errorf("项目不能为空")
	}

	// 检查仓库目录是否已存在且有效（有 HEAD 引用的分支文件存在）
	if r.isValidRepo(repoDir) {
		r.publish(taskID, taskEventTypeLog, "git_skip", model.TaskStatusRunning, "仓库目录已存在且有效，跳过 clone", nil)
		return nil
	}
	// 清理可能存在的残留目录（上次 clone 失败留下的半成品）
	_ = os.RemoveAll(repoDir)

	if project.GitRepoURL == "" {
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return fmt.Errorf("创建本地仓库目录失败: %w", err)
		}
		r.publish(taskID, taskEventTypeLog, "git_init", model.TaskStatusRunning, "项目未配置 Git 仓库地址，使用 git init 创建空仓库", nil)
		_ = r.runGitCommand(ctx, 0, filepath.Dir(repoDir), "init", "--initial-branch", projectDefaultBranch(project), repoDir)
		return nil
	}

	sharedRepoDir := filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))
	if err := r.prepareSharedRepository(ctx, taskID, project, branch, sharedRepoDir); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return fmt.Errorf("创建任务工作区父目录失败: %w", err)
	}

	_ = r.runGitCommand(ctx, taskID, sharedRepoDir, "worktree", "prune")
	ref := "origin/" + branch
	r.publish(taskID, taskEventTypeLog, "git_worktree_add", model.TaskStatusRunning,
		fmt.Sprintf("基于共享仓库创建任务工作树 (branch=%s)", branch), map[string]any{
			"shared_repo": sharedRepoDir,
			"worktree":    repoDir,
			"ref":         ref,
		})
	if err := r.runGitCommand(ctx, taskID, sharedRepoDir, "worktree", "add", "--force", repoDir, ref); err != nil {
		return fmt.Errorf("创建任务工作树失败 (branch=%s): %w", branch, err)
	}

	r.publish(taskID, taskEventTypeLog, "git_worktree_ready", model.TaskStatusRunning,
		"任务工作树已就绪", map[string]any{
			"shared_repo": sharedRepoDir,
			"worktree":    repoDir,
		})
	return nil
}

func (r *CLIRuntime) prepareNodeModules(ctx context.Context, taskID uint64, task *model.TestTask, repoDir, sharedNodeModulesDir, sharedRepoDir string) error {
	packageDirs, err := r.findPackageDirs(repoDir)
	if err != nil {
		r.publish(taskID, taskEventTypeLog, "node_modules_skip", model.TaskStatusRunning,
			"未找到package.json，跳过node_modules处理", nil)
		return nil
	}

	r.publish(taskID, taskEventTypeLog, "node_modules_scan_start", model.TaskStatusRunning,
		"开始扫描仓库内可复用的node_modules", map[string]any{
			"package_dir_count": len(packageDirs),
			"repo_dir":          repoDir,
		})

	for _, nodeModulesBaseDir := range packageDirs {
		packageJsonPath := filepath.Join(nodeModulesBaseDir, "package.json")
		nodeModulesDir := filepath.Join(nodeModulesBaseDir, "node_modules")
		if r.isValidNodeModules(nodeModulesDir) {
			r.publish(taskID, taskEventTypeLog, "node_modules_existing", model.TaskStatusRunning,
				"检测到已存在node_modules，直接复用", map[string]any{
					"node_modules": nodeModulesDir,
					"package_json": packageJsonPath,
				})
			return nil
		}

		if relPath, relErr := filepath.Rel(repoDir, nodeModulesBaseDir); relErr == nil {
			if !filepath.IsAbs(relPath) && relPath != ".." && !strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
				sharedRepoBaseDir := filepath.Join(sharedRepoDir, relPath)
				sharedRepoNodeModulesPath := filepath.Join(sharedRepoBaseDir, "node_modules")
				if r.isValidNodeModules(sharedRepoNodeModulesPath) {
					r.publish(taskID, taskEventTypeLog, "node_modules_reuse_shared_repo", model.TaskStatusRunning,
						"复用共享仓库中的node_modules", map[string]any{
							"shared_repo_node_modules": sharedRepoNodeModulesPath,
							"target_node_modules":      nodeModulesDir,
							"package_json":             packageJsonPath,
						})
					if err := r.copyNodeModules(sharedRepoNodeModulesPath, nodeModulesDir); err != nil {
						return fmt.Errorf("复制共享仓库node_modules失败: %w", err)
					}
					return nil
				}
			}
		}

		// 检查共享node_modules是否存在
		sharedNodeModulesPath := filepath.Join(sharedNodeModulesDir, filepath.Base(nodeModulesBaseDir))
		if r.isValidNodeModules(sharedNodeModulesPath) {
			r.publish(taskID, taskEventTypeLog, "node_modules_reuse", model.TaskStatusRunning,
				"复用共享node_modules", map[string]any{
					"shared_node_modules": sharedNodeModulesPath,
					"target_node_modules": nodeModulesDir,
					"package_json":        packageJsonPath,
				})
			// 复制共享node_modules到当前工作区
			if err := r.copyNodeModules(sharedNodeModulesPath, nodeModulesDir); err != nil {
				return fmt.Errorf("复制共享node_modules失败: %w", err)
			}
			return nil
		}
	}

	r.publish(taskID, taskEventTypeLog, "node_modules_skip_no_existing", model.TaskStatusRunning,
		"未发现已存在node_modules，已按配置跳过自动安装", map[string]any{
			"package_dir_count": len(packageDirs),
			"package_dirs":      packageDirs,
			"shared_repo":       sharedRepoDir,
			"shared_cache_dir":  sharedNodeModulesDir,
		})

	return nil
}

func (r *CLIRuntime) findPackageDirs(repoDir string) ([]string, error) {
	const maxWalkDepth = 10
	packageDirs := make([]string, 0, 4)
	seen := make(map[string]struct{})

	walkErr := filepath.WalkDir(repoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".idea" || name == ".vscode" {
				return filepath.SkipDir
			}
			if relPath != "." {
				depth := strings.Count(relPath, string(os.PathSeparator)) + 1
				if depth > maxWalkDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.Name() == "package.json" {
			dir := filepath.Dir(path)
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				packageDirs = append(packageDirs, dir)
			}
		}

		return nil
	})

	if walkErr != nil {
		return nil, fmt.Errorf("查找package.json失败: %w", walkErr)
	}
	if len(packageDirs) == 0 {
		return nil, fmt.Errorf("未找到package.json")
	}

	return packageDirs, nil
}

func (r *CLIRuntime) findPackageJson(repoDir string) (string, string, error) {
	// 从仓库根目录开始查找package.json
	currentDir := repoDir
	maxDepth := 10 // 防止无限递归

	for i := 0; i < maxDepth; i++ {
		packageJsonPath := filepath.Join(currentDir, "package.json")
		if _, err := os.Stat(packageJsonPath); err == nil {
			return packageJsonPath, currentDir, nil
		}

		// 如果已经到达根目录，停止查找
		if currentDir == filepath.Dir(currentDir) {
			break
		}

		// 向上移动到父目录
		currentDir = filepath.Dir(currentDir)
	}

	// 仓库内递归查找 package.json，支持深层目录（例如 docs/test/mobileapp/UAT Test/A7）。
	// 为了避免扫描过慢，跳过依赖目录和隐藏构建目录，并限制最大深度。
	const maxWalkDepth = 8
	var firstMatch string

	walkErr := filepath.WalkDir(repoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".idea" || name == ".vscode" {
				return filepath.SkipDir
			}
			if relPath != "." {
				depth := strings.Count(relPath, string(os.PathSeparator)) + 1
				if depth > maxWalkDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.Name() == "package.json" {
			firstMatch = path
			return io.EOF // 提前结束遍历
		}

		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, io.EOF) {
		return "", "", fmt.Errorf("递归查找package.json失败: %w", walkErr)
	}

	if firstMatch != "" {
		return firstMatch, filepath.Dir(firstMatch), nil
	}

	return "", "", fmt.Errorf("未找到package.json")
}

func (r *CLIRuntime) isValidNodeModules(dir string) bool {
	// node_modules 目录必须存在且为目录
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	// 常见锁文件位于 package 根目录而非 node_modules 目录。
	packageRoot := filepath.Dir(dir)
	lockFiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
	for _, name := range lockFiles {
		if _, err := os.Stat(filepath.Join(packageRoot, name)); err == nil {
			return true
		}
	}

	// 无锁文件时，存在已安装包目录也视为有效。
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// 作用域目录（如 @types）本身不算具体包，继续向下看。
		if strings.HasPrefix(name, "@") {
			scopedEntries, scopedErr := os.ReadDir(filepath.Join(dir, name))
			if scopedErr != nil {
				continue
			}
			for _, scoped := range scopedEntries {
				if scoped.IsDir() {
					return true
				}
			}
			continue
		}
		return true
	}

	return false
}

func (r *CLIRuntime) copyNodeModules(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("源node_modules不存在: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源node_modules不是目录: %s", src)
	}

	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("清理目标node_modules失败: %w", err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("创建目标node_modules目录失败: %w", err)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("robocopy", src, dst, "/E", "/NFL", "/NDL", "/NJH", "/NJS", "/NC", "/NS", "/NP")
		err := cmd.Run()
		if err == nil {
			return nil
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// robocopy 返回码约定：0-7 都表示成功或有差异复制完成，>=8 才是失败
			if code := exitErr.ExitCode(); code >= 0 && code < 8 {
				return nil
			}
			return fmt.Errorf("robocopy 复制node_modules失败, exit_code=%d", exitErr.ExitCode())
		}
		return fmt.Errorf("执行robocopy失败: %w", err)
	}

	cmd := exec.Command("cp", "-a", filepath.Join(src, "."), dst)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行cp复制node_modules失败: %w", err)
	}
	return nil
}

func (r *CLIRuntime) saveToSharedNodeModules(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("创建共享node_modules目录失败: %w", err)
	}

	// 复制node_modules到共享目录
	return r.copyNodeModules(src, dst)
}

func (r *CLIRuntime) prepareSharedRepository(ctx context.Context, taskID uint64, project *model.Project, branch, sharedRepoDir string) error {
	if r.isValidRepo(sharedRepoDir) {
		r.publish(taskID, taskEventTypeLog, "git_fetch", model.TaskStatusRunning,
			fmt.Sprintf("复用共享仓库并更新分支 (branch=%s)", branch), map[string]any{
				"shared_repo": sharedRepoDir,
			})
		if err := r.runGitCommand(ctx, taskID, sharedRepoDir, "fetch", "--depth", "1", "origin", branch); err != nil {
			return fmt.Errorf("更新共享仓库失败 (branch=%s): %w", branch, err)
		}
		return nil
	}

	_ = os.RemoveAll(sharedRepoDir)
	parentDir := filepath.Dir(sharedRepoDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("创建共享仓库父目录失败: %w", err)
	}

	repoURL := r.withGitCredentials(project.GitRepoURL)
	r.publish(taskID, taskEventTypeLog, "git_clone_shared", model.TaskStatusRunning,
		fmt.Sprintf("首次克隆共享仓库 (branch=%s)：%s", branch, project.GitRepoURL), map[string]any{
			"shared_repo": sharedRepoDir,
		})

	if err := r.runGitCommand(ctx, taskID, parentDir,
		"clone", "-c", "core.longpaths=true", "--depth", "1", "--single-branch", "-b", branch, repoURL, sharedRepoDir); err != nil {
		_ = os.RemoveAll(sharedRepoDir)
		return fmt.Errorf("克隆共享仓库失败 (branch=%s): %w", branch, err)
	}

	r.publish(taskID, taskEventTypeLog, "git_clone_shared_done", model.TaskStatusRunning, "共享仓库克隆完成", map[string]any{
		"shared_repo": sharedRepoDir,
	})

	return nil
}

// isValidRepo 检查 repoDir 是否是一个有效的 git 仓库（不只是目录存在）
func (r *CLIRuntime) isValidRepo(repoDir string) bool {
	gitDir := filepath.Join(repoDir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return false
	}
	// 检查 HEAD 文件存在且引用的分支有实际提交（排除 clone 失败的空仓库）
	headData, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return false
	}
	head := strings.TrimSpace(string(headData))
	if strings.HasPrefix(head, "ref: ") {
		refPath := filepath.Join(gitDir, strings.TrimPrefix(head, "ref: "))
		if _, err := os.Stat(refPath); err != nil {
			// 也检查 packed-refs（有些 clone 把 ref 打包了）
			packedRefs := filepath.Join(gitDir, "packed-refs")
			if _, err := os.Stat(packedRefs); err != nil {
				return false
			}
		}
	}
	return true
}

func (r *CLIRuntime) withGitCredentials(repoURL string) string {
	if r.settingRepo == nil {
		return repoURL
	}
	token := strings.TrimSpace(r.settingRepo.GetValue("gitlab", "access_token"))
	baseURL := strings.TrimSpace(r.settingRepo.GetValue("gitlab", "base_url"))
	if token == "" || baseURL == "" {
		return repoURL
	}

	baseParsed, err := parseGitURL(baseURL)
	if err != nil {
		return repoURL
	}
	repoParsed, err := parseGitURL(repoURL)
	if err != nil || !strings.EqualFold(baseParsed.Host, repoParsed.Host) {
		return repoURL
	}

	repoParsed.User = modelGitUserPassword(token)
	return repoParsed.String()
}

func parseGitURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func modelGitUserPassword(token string) *url.Userinfo {
	return url.UserPassword("oauth2", token)
}

func (r *CLIRuntime) writeControlFiles(
	workspace *CLIRuntimeWorkspace,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) error {
	if err := writeJSONFile(workspace.InputFile, input); err != nil {
		return fmt.Errorf("写入 CLI 输入文件失败: %w", err)
	}
	prompt := r.buildPrompt(workspace, task, input, workflow, agent, promptCtx)
	if err := os.WriteFile(workspace.PromptFile, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("写入 CLI Prompt 文件失败: %w", err)
	}
	if agent != nil {
		mcpPath := filepath.Join(workspace.ControlDir, "mcp_servers.json")
		if err := writeJSONFile(mcpPath, agent.MCPServers); err != nil {
			return fmt.Errorf("写入 MCP 配置文件失败: %w", err)
		}
	}
	return nil
}

func (r *CLIRuntime) buildPrompt(
	workspace *CLIRuntimeWorkspace,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) string {
	workflowPrompt := ""
	if workflow != nil && strings.TrimSpace(workflow.PromptTemplate) != "" {
		workflowPrompt = "\n## Workflow Prompt Template\n" + strings.TrimSpace(workflow.PromptTemplate) + "\n"
	}
	agentNote := ""
	if agent != nil {
		agentNote = fmt.Sprintf("\n## Agent\n- name: %s\n- description: %s\n", agent.Name, strings.TrimSpace(agent.Description))
	}
	mcpNote := "\n## MCP 预检\n- 当前未发现可用的 MCP 能力摘要，若 CLI 内部已注入 MCP，可自行探测后使用。\n"
	if promptCtx != nil {
		parts := make([]string, 0, 4)
		if len(promptCtx.ChromeMCPServers) > 0 {
			parts = append(parts, fmt.Sprintf("- 已接入 Chrome MCP Server: %s", strings.Join(promptCtx.ChromeMCPServers, ", ")))
		}
		if summary := strings.TrimSpace(promptCtx.MCPCapabilitySummary); summary != "" {
			parts = append(parts, "- MCP 能力摘要:")
			parts = append(parts, summary)
		}
		if len(parts) > 0 {
			mcpNote = "\n## MCP 预检\n" + strings.Join(parts, "\n") + "\n"
		}
	}
	return fmt.Sprintf(`# AutoTestFlow CLI Runtime

你正在执行 AutoTestFlow 的 gen-test 任务，请在当前仓库目录内使用 CLI Agent 完成测试资产生成。

## 必做事项
- 优先使用 gen-test 技能。
- 问题单上下文请读取 JSON 文件：%s
- 当前仓库根目录：%s
- 你可以在仓库内探索现有测试框架、共享工具、fixture 和文档目录。
- 如已配置 MCP，请使用本地 CLI 的 MCP 能力完成探索，不要假设项目结构。
- 依赖处理优先复用仓库内已有 node_modules；不要默认执行安装。
- 若确实需要安装依赖，只能使用 pnpm（pnpm install / pnpm add），禁止 npm install。
- 必须把实际的测试脚本和测试文档写入仓库目录。
- 生成的测试脚本必须包含至少一个可执行断言（例如 expect/assert/should 等）。
- 必须严格遵循 gen-test 技能流程推进：先探索项目中的测试脚本/测试文档/共享工具/测试数据，再生成测试文档与脚本，随后执行自测、定位失败、修复并重试。
- 自测循环最多 10 次；每次循环都要总结失败原因、修复动作和下一次验证点。
- 如果当前 Agent 可用 Chrome MCP，优先在涉及页面探索、自定义组件定位、DOM 结构确认时使用 Chrome MCP，而不是凭空假设选择器。
- 最终必须把结构化结果写入 JSON 文件：%s

## 结果 JSON 格式
{
  "test_cases": [
    {
      "title": "用例标题",
      "category": "main_flow",
      "precondition": "前置条件",
      "steps": "步骤1\n步骤2",
      "expected": "预期结果",
      "self_test_result": "pass",
      "priority": 1
    }
  ],
  "test_script": {
    "file_path": "tests/issue-xxx.spec.ts",
    "file_content": "",
    "language": "typescript"
  },
  "test_doc": {
    "title": "测试文档标题",
    "file_path": "docs/issue-xxx-test-case.md",
    "content": ""
  },
  "self_test": {
    "passed": true,
    "summary": "自测结论",
		"checks": ["检查项"],
		"playwright": {
			"passed": true,
			"summary": "Playwright 自测结论",
			"checks": ["Playwright 检查项"],
			"report_path": "playwright-report/index.html"
		},
		"midscene": {
			"passed": true,
			"summary": "Midscene 自测结论",
			"checks": ["Midscene 检查项"],
			"report_path": "reports/midscene-report.md"
		}
  },
  "summary": "生成总结"
}

说明：
- 如果文件已经直接写入仓库，可把 file_content/content 留空，系统会按 file_path 读取文件内容。
- file_path 必须是仓库内相对路径。
- test_script.file_content（或 file_path 指向的脚本）必须包含至少一个断言语句，禁止只写操作步骤不写校验。
- self_test.playwright 与 self_test.midscene 必须填写；若当前任务不适用，请写明 passed=false 与原因。
- 若执行失败，请尽量在 result.json 中写出已知错误上下文后退出。

## 当前任务
- task_id: %d
- issue_id: %d
- project: %s
- issue_title: %s
%s%s%s`,
		workspace.InputFile,
		workspace.RepoDir,
		workspace.ResultFile,
		task.ID,
		task.IssueID,
		task.Project.Name,
		input.IssueTitle,
		workflowPrompt,
		mcpNote,
		agentNote,
	)
}

func (r *CLIRuntime) executeCommand(ctx context.Context, runtimeCfg CLIRuntimeConfig, workspace *CLIRuntimeWorkspace, task *model.TestTask, input *GenTestInput, workflow *model.Skill, agent *model.Agent) error {
	callCtx, cancel := context.WithTimeout(ctx, runtimeCfg.Timeout)
	defer cancel()

	templateData := map[string]string{
		"workspace_dir": workspace.RootDir,
		"repo_dir":      workspace.RepoDir,
		"control_dir":   workspace.ControlDir,
		"input_file":    workspace.InputFile,
		"prompt_file":   workspace.PromptFile,
		"result_file":   workspace.ResultFile,
		"log_file":      workspace.LogFile,
		"task_id":       fmt.Sprintf("%d", task.ID),
		"issue_id":      fmt.Sprintf("%d", task.IssueID),
		"project_id":    fmt.Sprintf("%d", task.ProjectID),
		"workflow_name": task.SkillName,
		"agent_name":    safeAgentName(agent),
		"project_name":  task.Project.Name,
		"issue_title":   input.IssueTitle,
	}

	args := make([]string, 0, len(runtimeCfg.Args))
	for _, arg := range runtimeCfg.Args {
		args = append(args, renderTemplate(arg, templateData))
	}

	cmd := exec.CommandContext(callCtx, runtimeCfg.Command, args...)
	cmd.Dir = workspace.RepoDir
	cmd.Env = append(os.Environ(), buildCLIEnv(runtimeCfg.Env, templateData)...)

	// 将 prompt.md 作为 stdin 传给 CLI（支持 claude --print 等需要 stdin 输入的工具）
	promptFile, err := os.Open(workspace.PromptFile)
	if err != nil {
		return fmt.Errorf("打开 Prompt 文件失败: %w", err)
	}
	defer promptFile.Close()
	cmd.Stdin = promptFile

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 CLI stdout 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 CLI stderr 失败: %w", err)
	}

	cmdLine := runtimeCfg.Command
	if len(args) > 0 {
		cmdLine += " " + strings.Join(args, " ")
	}
	r.publish(task.ID, taskEventTypeStage, "cli_started", model.TaskStatusRunning,
		fmt.Sprintf("CLI 命令已启动: %s\n  工作目录: %s\n  日志文件: %s", cmdLine, workspace.RepoDir, workspace.LogFile),
		map[string]any{
			"command":  runtimeCfg.Command,
			"args":     args,
			"work_dir": workspace.RepoDir,
			"log_file": workspace.LogFile,
		})

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 CLI 命令失败: %w", err)
	}

	var (
		logMu   sync.Mutex
		logText strings.Builder
		wg      sync.WaitGroup
	)
	appendLine := func(line string) {
		logMu.Lock()
		defer logMu.Unlock()
		logText.WriteString(line)
		logText.WriteByte('\n')
	}

	wg.Add(2)
	go r.streamPipe(task.ID, "stdout", stdout, &wg, appendLine)
	go r.streamPipe(task.ID, "stderr", stderr, &wg, appendLine)

	waitErr := cmd.Wait()
	wg.Wait()

	logMu.Lock()
	logOutput := logText.String()
	logMu.Unlock()
	_ = os.WriteFile(workspace.LogFile, []byte(logOutput), 0o644)

	if waitErr != nil {
		r.publish(task.ID, taskEventTypeError, "cli_failed", model.TaskStatusFailed, "CLI 命令执行失败", map[string]any{
			"log_file": workspace.LogFile,
		})
		return fmt.Errorf("CLI Runtime 执行失败: %w，日志文件: %s", waitErr, workspace.LogFile)
	}
	return nil
}

func (r *CLIRuntime) streamPipe(taskID uint64, streamName string, pipe io.ReadCloser, wg *sync.WaitGroup, appendLine func(string)) {
	defer wg.Done()
	defer pipe.Close()

	reader := bufio.NewReaderSize(pipe, 64*1024)

	var rawLine string

	// 文本聚合：stream-json 的 text_delta 是逐 token 的碎片，
	// 攒够一行（遇到换行）或超过 200 字符再推送，避免事件风暴。
	var textBuf strings.Builder
	flushTextBuf := func() {
		if textBuf.Len() == 0 {
			return
		}
		text := strings.TrimRight(textBuf.String(), "\n")
		textBuf.Reset()
		if text == "" {
			return
		}
		r.publish(taskID, taskEventTypeLog, "cli_output", model.TaskStatusRunning, text, map[string]any{
			"stream": streamName,
		})
	}

	for {
		lineChunk, err := reader.ReadString('\n')
		rawLine = strings.TrimRight(lineChunk, "\r\n")
		line := strings.TrimSpace(rawLine)
		if line != "" {
			// codex --json 会先输出这类前导提示，属于噪音，直接忽略。
			if strings.EqualFold(line, "Reading prompt from stdin...") {
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}

			appendLine(rawLine)

			// 尝试解析 Claude/Codex 的 JSONL 输出
			if display, eventType, ok := parseStreamJSONLine(line); ok {
				if strings.HasPrefix(eventType, "codex_") {
					r.publish(taskID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, line, map[string]any{
						"stream": streamName,
					})
				}
				if eventType == "ai_question" || eventType == "permission_request" {
					flushTextBuf()
					if display != "" {
						r.publish(taskID, taskEventTypeLog, "cli_output", model.TaskStatusRunning, display, map[string]any{
							"stream": streamName,
						})
					}
					r.handleInteractionEvent(taskID, rawLine, eventType, display)
					if errors.Is(err, io.EOF) {
						break
					}
					continue
				}
				if eventType == "text_delta" {
					textBuf.WriteString(display)
					if strings.Contains(display, "\n") || textBuf.Len() >= 200 {
						flushTextBuf()
					}
				} else {
					flushTextBuf()
					if display != "" {
						r.publish(taskID, taskEventTypeLog, "cli_output", model.TaskStatusRunning, display, map[string]any{
							"stream": streamName,
						})
					}
				}
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}

			// 非 JSON 行（stderr 或普通文本），flush 后直接输出
			flushTextBuf()
			r.publish(taskID, taskEventTypeLog, "cli_output", model.TaskStatusRunning, line, map[string]any{
				"stream": streamName,
			})
		}

		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			r.publish(taskID, taskEventTypeLog, "cli_output_error", model.TaskStatusRunning, fmt.Sprintf("%s stream read error: %v", streamName, err), map[string]any{
				"stream": streamName,
			})
			break
		}
	}
	flushTextBuf()
}

// parseStreamJSONLine 解析 Claude/Codex 的单行 JSON 输出。
// 返回 (display, eventType, true)：display 为人类可读摘要，eventType 用于区分
// "text_delta"（需要聚合）和其他事件（立即推送）。display 为空表示该行可静默跳过。
func parseStreamJSONLine(line string) (string, string, bool) {
	if len(line) == 0 || line[0] != '{' {
		return "", "", false
	}

	if display, eventType, ok := parseCodexJSONLine(line); ok {
		return display, eventType, true
	}

	var event struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Message string `json:"message"`

		// result 事件
		Result     string  `json:"result"`
		CostUSD    float64 `json:"cost_usd"`
		DurationMS float64 `json:"duration_ms"`
		IsError    bool    `json:"is_error"`

		// content_block_delta
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`

		// content_block_start
		ContentBlock struct {
			Type string `json:"type"`
			Text string `json:"text"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", false
	}

	switch event.Type {
	case "system":
		if event.Message != "" {
			if event.Subtype == "init" {
				return fmt.Sprintf("🚀 初始化会话 - 工作目录: %s", extractCWD(event.Message)), "system_init", true
			}
			return fmt.Sprintf("ℹ️  %s", event.Message), "system", true
		}
		if event.Subtype != "" {
			return fmt.Sprintf("ℹ️  %s", event.Subtype), "system", true
		}
		return "", "system", true

	case "assistant":
		return parseAssistantMessage(line)

	case "user":
		return parseUserMessage(line)

	case "content_block_start":
		switch event.ContentBlock.Type {
		case "tool_use":
			toolName := event.ContentBlock.Name
			var prefix string
			switch toolName {
			case "AskUserQuestion":
				prefix = "🤔 AI 询问:"
			case "Read", "Glob", "Grep":
				prefix = "📖 读取文件:"
			case "Write", "Edit":
				prefix = "✏️  编辑文件:"
			case "WebSearch", "WebFetch":
				prefix = "🌐 网络请求:"
			default:
				prefix = "🔧 调用工具:"
			}
			return fmt.Sprintf("%s %s", prefix, toolName), "tool", true
		case "text":
			return "", "block_start", true
		default:
			return "", "block_start", true
		}

	case "content_block_delta":
		if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			return event.Delta.Text, "text_delta", true
		}
		return "", "delta", true

	case "content_block_stop":
		return "", "block_stop", true

	case "message_start", "message_delta", "message_stop":
		return "", "message", true

	case "result":
		if event.IsError {
			return fmt.Sprintf("❌ %s", formatError(event.Result)), "error", true
		}
		cost := ""
		if event.CostUSD > 0 {
			cost = fmt.Sprintf(" (💰 $%.4f, ⏱️ %.1fs)", event.CostUSD, event.DurationMS/1000)
		}
		return fmt.Sprintf("✅ 完成%s", cost), "result", true

	default:
		if event.Message != "" {
			return fmt.Sprintf("[%s] %s", event.Type, event.Message), event.Type, true
		}
		return fmt.Sprintf("[%s]", event.Type), event.Type, true
	}
	return "", "", false
}

func parseCodexJSONLine(line string) (string, string, bool) {
	var envelope struct {
		Type string          `json:"type"`
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return "", "", false
	}

	if envelope.Type == "" || !strings.Contains(envelope.Type, ".") {
		return "", "", false
	}

	if strings.HasPrefix(envelope.Type, "thread.") || strings.HasPrefix(envelope.Type, "turn.") {
		switch envelope.Type {
		case "thread.started":
			return "🚀 Codex 会话已启动", "codex_thread", true
		case "thread.completed":
			return "✅ Codex 会话已完成", "codex_thread", true
		default:
			return "", "codex_lifecycle", true
		}
	}

	if !strings.HasPrefix(envelope.Type, "item.") {
		return "", "", false
	}

	var item struct {
		Type             string `json:"type"`
		Text             string `json:"text"`
		Command          string `json:"command"`
		Status           string `json:"status"`
		ExitCode         *int   `json:"exit_code"`
		AggregatedOutput string `json:"aggregated_output"`
		Items            []struct {
			Text      string `json:"text"`
			Completed bool   `json:"completed"`
		} `json:"items"`
	}
	if len(envelope.Item) > 0 {
		if err := json.Unmarshal(envelope.Item, &item); err != nil {
			return "", "codex_item", true
		}
	}

	switch item.Type {
	case "reasoning":
		if strings.HasPrefix(envelope.Type, "item.completed") && strings.TrimSpace(item.Text) != "" {
			return fmt.Sprintf("🧠 %s", truncateText(strings.Split(strings.TrimSpace(item.Text), "\n")[0], 160)), "codex_reasoning", true
		}
		return "", "codex_reasoning", true

	case "todo_list":
		if len(item.Items) == 0 {
			return "", "codex_todo", true
		}
		completed := 0
		for _, todo := range item.Items {
			if todo.Completed {
				completed++
			}
		}
		return fmt.Sprintf("🗂️ 任务进度: %d/%d", completed, len(item.Items)), "codex_todo", true

	case "command_execution":
		if strings.HasPrefix(envelope.Type, "item.completed") {
			if item.ExitCode != nil && *item.ExitCode != 0 {
				msg := "❌ 命令执行失败"
				if strings.TrimSpace(item.Command) != "" {
					msg += ": " + truncateText(item.Command, 120)
				}
				if out := strings.TrimSpace(item.AggregatedOutput); out != "" {
					msg += " | " + truncateText(out, 180)
				}
				return msg, "codex_command", true
			}
			return "", "codex_command", true
		}
		return "", "codex_command", true

	case "message":
		if strings.TrimSpace(item.Text) != "" {
			return fmt.Sprintf("💬 %s", truncateText(strings.TrimSpace(item.Text), 300)), "codex_message", true
		}
		return "", "codex_message", true

	default:
		return "", "codex_item", true
	}
}

func parseAssistantMessage(line string) (string, string, bool) {
	var assistant struct {
		Message struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Role       string `json:"role"`
			Model      string `json:"model"`
			Content    []any  `json:"content"`
			StopReason string `json:"stop_reason"`
		} `json:"message"`
	}

	if err := json.Unmarshal([]byte(line), &assistant); err != nil {
		return "", "", false
	}

	for _, item := range assistant.Message.Content {
		content, ok := item.(map[string]any)
		if !ok {
			continue
		}

		contentType, _ := content["type"].(string)

		switch contentType {
		case "thinking":
			if thinking, ok := content["thinking"].(string); ok && thinking != "" {
				lines := strings.Split(strings.TrimSpace(thinking), "\n")
				if len(lines) > 0 {
					return fmt.Sprintf("🧠 %s", strings.Join(lines[:1], "")), "thinking", true
				}
			}
		case "text":
			if text, ok := content["text"].(string); ok && text != "" {
				lines := strings.Split(strings.TrimSpace(text), "\n")
				if len(lines) > 0 {
					return fmt.Sprintf("💬 %s", strings.Join(lines[:1], "")), "assistant_text", true
				}
			}
		case "tool_use":
			if name, ok := content["name"].(string); ok {
				if name == "AskUserQuestion" {
					if input, ok := content["input"].(map[string]any); ok {
						if question, ok := input["question"].(string); ok {
							return fmt.Sprintf("❓ AI 提问: %s", question), "ai_question", true
						}
					}
				}
			}
		}
	}

	return "", "assistant", true
}

func parseUserMessage(line string) (string, string, bool) {
	var user struct {
		Message struct {
			Role    string `json:"role"`
			Content []any  `json:"content"`
		} `json:"message"`
	}

	if err := json.Unmarshal([]byte(line), &user); err != nil {
		return "", "", false
	}

	for _, item := range user.Message.Content {
		content, ok := item.(map[string]any)
		if !ok {
			continue
		}

		contentType, _ := content["type"].(string)

		switch contentType {
		case "tool_result":
			if toolResult, ok := content["content"].(string); ok {
				if strings.Contains(toolResult, "requested permissions") {
					return fmt.Sprintf("🔒 权限请求: %s", formatPermissionError(toolResult)), "permission_request", true
				}
				if isError, _ := content["is_error"].(bool); isError {
					return fmt.Sprintf("⚠️  工具错误: %s", formatError(toolResult)), "tool_error", true
				}
				return fmt.Sprintf("📋 结果: %s", truncateText(toolResult, 100)), "tool_result", true
			}
		}
	}

	return "", "user", true
}

func extractCWD(message string) string {
	re := regexp.MustCompile(`"cwd":"([^"]+)"`)
	matches := re.FindStringSubmatch(message)
	if len(matches) > 1 {
		cwd := matches[1]
		if len(cwd) > 50 {
			return "..." + cwd[len(cwd)-50:]
		}
		return cwd
	}
	return message
}

func formatError(err string) string {
	err = strings.TrimSpace(err)
	if strings.Contains(err, "File does not exist") {
		return "文件不存在"
	}
	if strings.Contains(err, "No files found") {
		return "未找到文件"
	}
	if strings.Contains(err, "permission") || strings.Contains(err, "Permission") {
		return formatPermissionError(err)
	}
	if len(err) > 100 {
		return err[:100] + "..."
	}
	return err
}

func formatPermissionError(err string) string {
	re := regexp.MustCompile(`Claude requested permissions to ([^.]+)\.?\s*(.+)?`)
	matches := re.FindStringSubmatch(err)
	if len(matches) > 2 {
		action := strings.TrimPrefix(matches[1], "read from ")
		action = strings.TrimPrefix(action, "write to ")
		return fmt.Sprintf("需要访问: %s", action)
	}
	return "需要权限访问"
}

func truncateText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func buildCLIEnv(envMap map[string]string, templateData map[string]string) []string {
	defaults := map[string]string{
		"ATF_WORKSPACE_DIR": templateData["workspace_dir"],
		"ATF_REPO_DIR":      templateData["repo_dir"],
		"ATF_CONTROL_DIR":   templateData["control_dir"],
		"ATF_INPUT_FILE":    templateData["input_file"],
		"ATF_PROMPT_FILE":   templateData["prompt_file"],
		"ATF_RESULT_FILE":   templateData["result_file"],
		"ATF_LOG_FILE":      templateData["log_file"],
		"ATF_TASK_ID":       templateData["task_id"],
		"ATF_ISSUE_ID":      templateData["issue_id"],
		"ATF_PROJECT_ID":    templateData["project_id"],
		"ATF_WORKFLOW_NAME": templateData["workflow_name"],
		"ATF_AGENT_NAME":    templateData["agent_name"],
		"ATF_PROJECT_NAME":  templateData["project_name"],
		"ATF_ISSUE_TITLE":   templateData["issue_title"],
	}

	result := make([]string, 0, len(defaults)+len(envMap))
	for key, value := range defaults {
		result = append(result, key+"="+value)
	}
	for key, value := range envMap {
		result = append(result, key+"="+renderTemplate(value, templateData))
	}
	return result
}

func (r *CLIRuntime) readResult(workspace *CLIRuntimeWorkspace) (*GenTestOutput, error) {
	data, err := os.ReadFile(workspace.ResultFile)
	if err != nil {
		return nil, fmt.Errorf("读取 CLI 结果文件失败: %w", err)
	}
	var output GenTestOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("解析 CLI 结果文件失败: %w", err)
	}
	return &output, nil
}

func (r *CLIRuntime) syncArtifacts(repoDir string, task *model.TestTask, input *GenTestInput, output *GenTestOutput) error {
	if output == nil {
		return fmt.Errorf("CLI 输出为空")
	}

	scriptPath := strings.TrimSpace(output.TestScript.FilePath)
	if scriptPath == "" && strings.TrimSpace(output.TestScript.FileContent) != "" {
		scriptPath = fmt.Sprintf("tests/issue-%d.spec.ts", task.IssueID)
	}
	if scriptPath != "" {
		scriptPath = normalizeRepoRelativePath(scriptPath)
		output.TestScript.FilePath = scriptPath
		if strings.TrimSpace(output.TestScript.Language) == "" {
			output.TestScript.Language = normalizeScriptLanguage(output.TestScript.Language, scriptPath)
		}
		if strings.TrimSpace(output.TestScript.FileContent) == "" {
			content, err := readRepoFile(repoDir, scriptPath)
			if err != nil {
				var notFound *artifactFileNotFoundError
				if errors.As(err, &notFound) {
					r.logger.Warn("测试脚本文件不存在，跳过读取",
						zap.String("path", notFound.Path),
						zap.String("relative_path", notFound.RelativePath))
				} else {
					return err
				}
			} else {
				output.TestScript.FileContent = content
			}
		} else if err := writeRepoFile(repoDir, scriptPath, output.TestScript.FileContent); err != nil {
			return err
		}
	}

	docPath := strings.TrimSpace(output.TestDoc.FilePath)
	if docPath == "" && strings.TrimSpace(output.TestDoc.Content) != "" {
		docPath = buildDefaultDocPath(task)
	}
	if docPath != "" {
		docPath = normalizeRepoRelativePath(docPath)
		output.TestDoc.FilePath = docPath
		if strings.TrimSpace(output.TestDoc.Content) == "" {
			content, err := readRepoFile(repoDir, docPath)
			if err != nil {
				var notFound *artifactFileNotFoundError
				if errors.As(err, &notFound) {
					r.logger.Warn("测试文档文件不存在，跳过读取",
						zap.String("path", notFound.Path),
						zap.String("relative_path", notFound.RelativePath))
				} else {
					return err
				}
			} else {
				output.TestDoc.Content = content
			}
		} else if err := writeRepoFile(repoDir, docPath, output.TestDoc.Content); err != nil {
			return err
		}
		if strings.TrimSpace(output.TestDoc.Title) == "" {
			output.TestDoc.Title = fmt.Sprintf("测试文档 - %s", input.IssueTitle)
		}
	}

	return nil
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

func renderTemplate(template string, data map[string]string) string {
	result := template
	for key, value := range data {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func safeAgentName(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	return agent.Name
}

func projectDefaultBranch(project *model.Project) string {
	if project != nil && strings.TrimSpace(project.GitBranch) != "" {
		return strings.TrimSpace(project.GitBranch)
	}
	return "main"
}

func sanitizePathComponent(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	return replacer.Replace(trimmed)
}

func (r *CLIRuntime) runGitCommand(ctx context.Context, taskID uint64, dir string, args ...string) error {
	// clone 操作给 10 分钟（大仓库需要较长时间），其他操作 2 分钟
	timeout := 2 * time.Minute
	if len(args) > 0 && args[0] == "clone" {
		timeout = 10 * time.Minute
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
		"GIT_ASKPASS=",
	)

	// 不需要实时输出时（taskID==0 或无 eventHub），走简单路径
	if taskID == 0 || r.eventHub == nil {
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s 失败: %w, output=%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	// 流式读取 stdout + stderr，实时发布到事件流
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 git stdout 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 git stderr 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 git %s 失败: %w", args[0], err)
	}

	var (
		wg       sync.WaitGroup
		outputMu sync.Mutex
		combined strings.Builder
	)
	collectAndPublish := func(streamName string, pipe io.ReadCloser) {
		defer wg.Done()
		defer pipe.Close()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		// git --progress 的进度行用 \r 而非 \n 分隔，需要同时按 \r 和 \n 拆分
		scanner.Split(scanCROrLF)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			outputMu.Lock()
			combined.WriteString(line)
			combined.WriteByte('\n')
			outputMu.Unlock()
			r.publish(taskID, taskEventTypeLog, "git_output", model.TaskStatusRunning, line, map[string]any{
				"stream": streamName,
			})
		}
	}

	wg.Add(2)
	go collectAndPublish("stdout", stdout)
	go collectAndPublish("stderr", stderr)

	waitErr := cmd.Wait()
	wg.Wait()

	if waitErr != nil {
		outputMu.Lock()
		out := combined.String()
		outputMu.Unlock()
		return fmt.Errorf("git %s 失败: %w, output=%s", strings.Join(args, " "), waitErr, strings.TrimSpace(out))
	}
	return nil
}

func (r *CLIRuntime) publish(taskID uint64, eventType, stage, status, message string, data map[string]any) {
	if r.eventHub != nil {
		r.eventHub.Publish(taskID, TaskEvent{
			Type:      eventType,
			Stage:     stage,
			Status:    status,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

func (r *CLIRuntime) handleInteractionEvent(taskID uint64, rawLine, eventType, display string) {
	interactionType := "ai_question"
	if eventType == "permission_request" {
		interactionType = "permission_request"
	}

	interaction := &model.CLIInteraction{
		TaskID:          uint(taskID),
		InteractionType: interactionType,
		Content:         display,
		Status:          "pending",
		Metadata:        model.JSON(`{"raw_line": "` + strings.ReplaceAll(rawLine, `"`, `\"`) + `"}`),
	}

	if err := r.interactionRepo.Create(interaction); err != nil {
		r.logger.Error("创建交互记录失败",
			zap.Uint64("task_id", taskID),
			zap.String("type", interactionType),
			zap.Error(err))
	}
}

// scanCROrLF 是 bufio.Scanner 的自定义 split 函数，同时按 \r 和 \n 拆行。
// git --progress 的进度输出使用 \r 覆盖当前行，标准 Scanner 只按 \n 切分会导致
// 整段进度信息攒成一大行，无法实时推送给前端。
func scanCROrLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			// 跳过 \r\n 组合，视为一个换行
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
