package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"auto-test-flow/internal/model"

	"go.uber.org/zap"
)

type GenTestWorkspaceConfig struct {
	WorkspaceRoot     string
	RepoDirName       string
	ControlDirName    string
	InputFileName     string
	PromptFileName    string
	ResultFileName    string
	LogFileName       string
	PreserveWorkspace bool
}

type GenTestWorkspaceService struct {
	logger      *zap.Logger
	eventHub    *TaskEventHub
	repoSupport *RuntimeRepoSupport
}

func NewGenTestWorkspaceService(logger *zap.Logger) *GenTestWorkspaceService {
	return &GenTestWorkspaceService{
		logger:      logger,
		eventHub:    DefaultTaskEventHub,
		repoSupport: NewRuntimeRepoSupport(logger, DefaultTaskEventHub),
	}
}

func ResolveGenTestWorkspaceConfig(agent *model.Agent) (GenTestWorkspaceConfig, error) {
	cfg, err := ResolveRuntimeWorkspaceConfig(agent)
	if err != nil {
		return GenTestWorkspaceConfig{}, err
	}
	return GenTestWorkspaceConfig(cfg), nil
}

func (s *GenTestWorkspaceService) Prepare(
	ctx context.Context,
	taskID uint64,
	task *model.TestTask,
	cfg GenTestWorkspaceConfig,
) (*RuntimeWorkspace, error) {
	projectDir := filepath.Join(cfg.WorkspaceRoot, fmt.Sprintf("project_%d", task.ProjectID))
	rootDir := filepath.Join(projectDir, fmt.Sprintf("task_%d", task.ID))
	repoDir := filepath.Join(rootDir, cfg.RepoDirName)
	controlDir := filepath.Join(rootDir, cfg.ControlDirName)
	branch := projectDefaultBranch(task.Project)
	sharedRepoDir := filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))
	sharedNodeModulesDir := filepath.Join(projectDir, "_shared", "node_modules")

	workspace := &RuntimeWorkspace{
		RootDir:           rootDir,
		RepoDir:           repoDir,
		ControlDir:        controlDir,
		InputFile:         filepath.Join(controlDir, cfg.InputFileName),
		PromptFile:        filepath.Join(controlDir, cfg.PromptFileName),
		ResultFile:        filepath.Join(controlDir, cfg.ResultFileName),
		LogFile:           filepath.Join(controlDir, cfg.LogFileName),
		SharedNodeModules: sharedNodeModulesDir,
	}

	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建运行时控制目录失败: %w", err)
	}
	if err := s.prepareRepository(ctx, taskID, task.Project, branch, projectDir, repoDir); err != nil {
		return nil, err
	}
	if err := s.prepareNodeModules(ctx, taskID, repoDir, sharedNodeModulesDir, sharedRepoDir); err != nil {
		return nil, err
	}
	return workspace, nil
}

func (s *GenTestWorkspaceService) prepareRepository(ctx context.Context, taskID uint64, project *model.Project, branch, projectDir, repoDir string) error {
	if project == nil {
		return fmt.Errorf("项目不能为空")
	}
	if s.repoSupport.IsValidRepo(repoDir) {
		s.publish(taskID, taskEventTypeLog, "git_skip", model.TaskStatusRunning, "仓库目录已存在且有效，跳过 clone", nil)
		return nil
	}
	_ = os.RemoveAll(repoDir)

	if project.GitRepoURL == "" {
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return fmt.Errorf("创建本地仓库目录失败: %w", err)
		}
		s.publish(taskID, taskEventTypeLog, "git_init", model.TaskStatusRunning, "项目未配置 Git 仓库地址，使用 git init 创建空仓库", nil)
		_ = s.repoSupport.RunGitCommand(ctx, 0, filepath.Dir(repoDir), "init", "--initial-branch", projectDefaultBranch(project), repoDir)
		return nil
	}

	sharedRepoDir := filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))
	if err := s.repoSupport.EnsureSharedRepository(ctx, taskID, project, branch, sharedRepoDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return fmt.Errorf("创建任务工作区父目录失败: %w", err)
	}

	_ = s.repoSupport.RunGitCommand(ctx, taskID, sharedRepoDir, "worktree", "prune")
	ref := "origin/" + branch
	s.publish(taskID, taskEventTypeLog, "git_worktree_add", model.TaskStatusRunning,
		fmt.Sprintf("基于共享仓库创建任务工作树 (branch=%s)", branch), map[string]any{
			"shared_repo": sharedRepoDir,
			"worktree":    repoDir,
			"ref":         ref,
		})
	if err := s.repoSupport.RunGitCommand(ctx, taskID, sharedRepoDir, "worktree", "add", "--force", repoDir, ref); err != nil {
		return fmt.Errorf("创建任务工作树失败 (branch=%s): %w", branch, err)
	}
	s.publish(taskID, taskEventTypeLog, "git_worktree_ready", model.TaskStatusRunning, "任务工作树已就绪", map[string]any{
		"shared_repo": sharedRepoDir,
		"worktree":    repoDir,
	})
	return nil
}

func (s *GenTestWorkspaceService) prepareNodeModules(ctx context.Context, taskID uint64, repoDir, sharedNodeModulesDir, sharedRepoDir string) error {
	packageDirs, err := s.findPackageDirs(repoDir)
	if err != nil {
		s.publish(taskID, taskEventTypeLog, "node_modules_skip", model.TaskStatusRunning, "未找到package.json，跳过node_modules处理", nil)
		return nil
	}

	s.publish(taskID, taskEventTypeLog, "node_modules_scan_start", model.TaskStatusRunning, "开始扫描仓库内可复用的node_modules", map[string]any{
		"package_dir_count": len(packageDirs),
		"repo_dir":          repoDir,
	})

	for _, nodeModulesBaseDir := range packageDirs {
		packageJSONPath := filepath.Join(nodeModulesBaseDir, "package.json")
		nodeModulesDir := filepath.Join(nodeModulesBaseDir, "node_modules")
		if s.isValidNodeModules(nodeModulesDir) {
			s.publish(taskID, taskEventTypeLog, "node_modules_existing", model.TaskStatusRunning, "检测到已存在node_modules，直接复用", map[string]any{
				"node_modules": nodeModulesDir,
				"package_json": packageJSONPath,
			})
			return nil
		}

		if relPath, relErr := filepath.Rel(repoDir, nodeModulesBaseDir); relErr == nil {
			if !filepath.IsAbs(relPath) && relPath != ".." && !strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
				sharedRepoBaseDir := filepath.Join(sharedRepoDir, relPath)
				sharedRepoNodeModulesPath := filepath.Join(sharedRepoBaseDir, "node_modules")
				if s.isValidNodeModules(sharedRepoNodeModulesPath) {
					s.publish(taskID, taskEventTypeLog, "node_modules_reuse_shared_repo", model.TaskStatusRunning, "创建连接复用共享仓库中的node_modules", map[string]any{
						"shared_repo_node_modules": sharedRepoNodeModulesPath,
						"target_node_modules":      nodeModulesDir,
						"package_json":             packageJSONPath,
					})
					if err := s.linkNodeModules(sharedRepoNodeModulesPath, nodeModulesDir); err != nil {
						return fmt.Errorf("创建共享仓库node_modules连接失败: %w", err)
					}
					return nil
				}
			}
		}

		sharedNodeModulesPath := filepath.Join(sharedNodeModulesDir, filepath.Base(nodeModulesBaseDir))
		if s.isValidNodeModules(sharedNodeModulesPath) {
			s.publish(taskID, taskEventTypeLog, "node_modules_reuse", model.TaskStatusRunning, "创建连接复用共享node_modules", map[string]any{
				"shared_node_modules": sharedNodeModulesPath,
				"target_node_modules": nodeModulesDir,
				"package_json":        packageJSONPath,
			})
			if err := s.linkNodeModules(sharedNodeModulesPath, nodeModulesDir); err != nil {
				return fmt.Errorf("创建共享node_modules连接失败: %w", err)
			}
			return nil
		}
	}

	s.publish(taskID, taskEventTypeLog, "node_modules_skip_no_existing", model.TaskStatusRunning, "未发现已存在node_modules，已按配置跳过自动安装", map[string]any{
		"package_dir_count": len(packageDirs),
		"package_dirs":      packageDirs,
		"shared_repo":       sharedRepoDir,
		"shared_cache_dir":  sharedNodeModulesDir,
	})
	return nil
}

func (s *GenTestWorkspaceService) findPackageDirs(repoDir string) ([]string, error) {
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

func (s *GenTestWorkspaceService) isValidNodeModules(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	packageRoot := filepath.Dir(dir)
	lockFiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
	for _, name := range lockFiles {
		if _, err := os.Stat(filepath.Join(packageRoot, name)); err == nil {
			return true
		}
	}

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

func (s *GenTestWorkspaceService) linkNodeModules(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("源node_modules不存在: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源node_modules不是目录: %s", src)
	}

	// 清理目标路径（可能是目录或符号链接）
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("清理目标node_modules失败: %w", err)
	}

	// 创建符号链接
	if runtime.GOOS == "windows" {
		// Windows 使用 junction（目录连接）
		cmd := exec.Command("cmd", "/c", "mklink", "/J", dst, src)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("创建目录连接失败: %w, output: %s", err, string(output))
		}
		return nil
	}

	// Linux/macOS 使用符号链接
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("创建符号链接失败: %w", err)
	}
	return nil
}

func (s *GenTestWorkspaceService) SyncArtifacts(repoDir string, task *model.TestTask, input *GenTestInput, output *GenTestOutput) error {
	if output == nil {
		return fmt.Errorf("运行时输出为空")
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
					s.logger.Warn("测试脚本文件不存在，跳过读取", zap.String("path", notFound.Path), zap.String("relative_path", notFound.RelativePath))
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
					s.logger.Warn("测试文档文件不存在，跳过读取", zap.String("path", notFound.Path), zap.String("relative_path", notFound.RelativePath))
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

func (s *GenTestWorkspaceService) WriteControlFiles(workspace *RuntimeWorkspace, input *GenTestInput, prompt string, agent *model.Agent) error {
	if err := writeJSONFile(workspace.InputFile, input); err != nil {
		return fmt.Errorf("写入运行时输入文件失败: %w", err)
	}
	if err := os.WriteFile(workspace.PromptFile, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("写入运行时 Prompt 文件失败: %w", err)
	}
	if agent != nil {
		mcpPath := filepath.Join(workspace.ControlDir, "mcp_servers.json")
		if err := writeJSONFile(mcpPath, agent.MCPServers); err != nil {
			return fmt.Errorf("写入 MCP 配置文件失败: %w", err)
		}
	}
	return nil
}

func (s *GenTestWorkspaceService) WriteResultFile(workspace *RuntimeWorkspace, output *GenTestOutput) error {
	return writeJSONFile(workspace.ResultFile, output)
}

func (s *GenTestWorkspaceService) publish(taskID uint64, eventType, stage, status, message string, data map[string]any) {
	if s.eventHub != nil {
		s.eventHub.Publish(taskID, TaskEvent{
			Type:      eventType,
			Stage:     stage,
			Status:    status,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}
