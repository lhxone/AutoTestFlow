package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

// GitPullService 周期性拉取项目共享仓库，保证 worktree 来源持续更新。
type GitPullService struct {
	projectRepo *repository.ProjectRepo
	cliRuntime  *CLIRuntime
	logger      *zap.Logger
}

func NewGitPullService(logger *zap.Logger) *GitPullService {
	return &GitPullService{
		projectRepo: repository.NewProjectRepo(),
		cliRuntime:  NewCLIRuntime(logger),
		logger:      logger,
	}
}

func (s *GitPullService) SyncDueProjects(ctx context.Context) {
	projects, err := s.projectRepo.GetAllActive()
	if err != nil {
		s.logger.Error("获取项目列表失败，跳过 Git 定时拉取", zap.Error(err))
		return
	}

	workspaceRoot := resolveWorkspaceRoot()
	now := time.Now()
	for i := range projects {
		project := &projects[i]
		if strings.TrimSpace(project.GitRepoURL) == "" {
			continue
		}
		if project.GitPullInterval <= 0 {
			continue
		}
		if project.GitLastPullAt != nil {
			nextPullAt := project.GitLastPullAt.Add(time.Duration(project.GitPullInterval) * time.Minute)
			if now.Before(nextPullAt) {
				continue
			}
		}

		if err := s.pullProjectSharedRepo(ctx, project, workspaceRoot); err != nil {
			s.logger.Error("项目共享仓库定时拉取失败",
				zap.Uint64("project_id", project.ID),
				zap.String("project_name", project.Name),
				zap.String("git_branch", projectDefaultBranch(project)),
				zap.Error(err))
			continue
		}

		pulledAt := time.Now()
		project.GitLastPullAt = &pulledAt
		if err := s.projectRepo.Update(project); err != nil {
			s.logger.Warn("更新项目 Git 最后拉取时间失败",
				zap.Uint64("project_id", project.ID),
				zap.Error(err))
		}
	}
}

func (s *GitPullService) pullProjectSharedRepo(ctx context.Context, project *model.Project, workspaceRoot string) error {
	if project == nil {
		return fmt.Errorf("project is nil")
	}

	branch := projectDefaultBranch(project)
	projectDir := filepath.Join(workspaceRoot, fmt.Sprintf("project_%d", project.ID))
	sharedRepoDir := filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))

	if _, err := os.Stat(filepath.Join(sharedRepoDir, ".git")); err == nil {
		if err := s.cliRuntime.runGitCommand(ctx, 0, sharedRepoDir, "fetch", "--depth", "1", "origin", branch); err != nil {
			return err
		}
		if err := s.cliRuntime.runGitCommand(ctx, 0, sharedRepoDir, "reset", "--hard", "origin/"+branch); err != nil {
			return err
		}
		s.logger.Info("项目共享仓库拉取完成",
			zap.Uint64("project_id", project.ID),
			zap.String("project_name", project.Name),
			zap.String("shared_repo", sharedRepoDir),
			zap.String("branch", branch))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(sharedRepoDir), 0o755); err != nil {
		return fmt.Errorf("创建共享仓库目录失败: %w", err)
	}

	repoURL := s.cliRuntime.withGitCredentials(project.GitRepoURL)
	if err := s.cliRuntime.runGitCommand(ctx, 0, filepath.Dir(sharedRepoDir),
		"clone", "-c", "core.longpaths=true", "--depth", "1", "--single-branch", "-b", branch, repoURL, sharedRepoDir); err != nil {
		return err
	}

	s.logger.Info("首次克隆项目共享仓库完成",
		zap.Uint64("project_id", project.ID),
		zap.String("project_name", project.Name),
		zap.String("shared_repo", sharedRepoDir),
		zap.String("branch", branch))
	return nil
}

func resolveWorkspaceRoot() string {
	raw := LoadCLIRuntimeConfig()
	workspaceRoot := strings.TrimSpace(raw.WorkspaceRoot)
	if workspaceRoot != "" {
		return workspaceRoot
	}
	return filepath.Join(config.Global.Git.WorkDir, "cli-runtime")
}
