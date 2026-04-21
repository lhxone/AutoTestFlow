package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type RuntimeRepoSupport struct {
	logger      *zap.Logger
	settingRepo *repository.SettingRepo
	eventHub    *TaskEventHub
}

func NewRuntimeRepoSupport(logger *zap.Logger, eventHub *TaskEventHub) *RuntimeRepoSupport {
	return &RuntimeRepoSupport{
		logger:      logger,
		settingRepo: repository.NewSettingRepo(),
		eventHub:    eventHub,
	}
}

func (s *RuntimeRepoSupport) SharedRepoDir(workspaceRoot string, projectID uint64, branch string) string {
	projectDir := filepath.Join(workspaceRoot, fmt.Sprintf("project_%d", projectID))
	return filepath.Join(projectDir, "_shared", "repo_"+sanitizePathComponent(branch))
}

func (s *RuntimeRepoSupport) EnsureSharedRepository(ctx context.Context, taskID uint64, project *model.Project, branch, sharedRepoDir string) error {
	if s.IsValidRepo(sharedRepoDir) {
		s.publish(taskID, taskEventTypeLog, "git_fetch", model.TaskStatusRunning, fmt.Sprintf("复用共享仓库并更新分支 (branch=%s)", branch), map[string]any{
			"shared_repo": sharedRepoDir,
		})
		if err := s.RunGitCommand(ctx, taskID, sharedRepoDir, "fetch", "--depth", "1", "origin", branch); err != nil {
			return fmt.Errorf("更新共享仓库失败 (branch=%s): %w", branch, err)
		}
		return nil
	}

	_ = os.RemoveAll(sharedRepoDir)
	parentDir := filepath.Dir(sharedRepoDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("创建共享仓库父目录失败: %w", err)
	}

	repoURL := s.WithGitCredentials(project.GitRepoURL)
	s.publish(taskID, taskEventTypeLog, "git_clone_shared", model.TaskStatusRunning, fmt.Sprintf("首次克隆共享仓库 (branch=%s)：%s", branch, project.GitRepoURL), map[string]any{
		"shared_repo": sharedRepoDir,
	})
	if err := s.RunGitCommand(ctx, taskID, parentDir, "clone", "-c", "core.longpaths=true", "--depth", "1", "--single-branch", "-b", branch, repoURL, sharedRepoDir); err != nil {
		_ = os.RemoveAll(sharedRepoDir)
		return fmt.Errorf("克隆共享仓库失败 (branch=%s): %w", branch, err)
	}
	s.publish(taskID, taskEventTypeLog, "git_clone_shared_done", model.TaskStatusRunning, "共享仓库克隆完成", map[string]any{
		"shared_repo": sharedRepoDir,
	})
	return nil
}

func (s *RuntimeRepoSupport) IsValidRepo(repoDir string) bool {
	gitDir := filepath.Join(repoDir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return false
	}

	headData, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return false
	}
	head := strings.TrimSpace(string(headData))
	if strings.HasPrefix(head, "ref: ") {
		refPath := filepath.Join(gitDir, strings.TrimPrefix(head, "ref: "))
		if _, err := os.Stat(refPath); err != nil {
			packedRefs := filepath.Join(gitDir, "packed-refs")
			if _, err := os.Stat(packedRefs); err != nil {
				return false
			}
		}
	}
	return true
}

func (s *RuntimeRepoSupport) WithGitCredentials(repoURL string) string {
	if s.settingRepo == nil {
		return repoURL
	}
	token := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "access_token"))
	baseURL := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "base_url"))
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

func (s *RuntimeRepoSupport) RunGitCommand(ctx context.Context, taskID uint64, dir string, args ...string) error {
	timeout := 2 * time.Minute
	if len(args) > 0 && args[0] == "clone" {
		timeout = 10 * time.Minute
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "GIT_ASKPASS=")

	if taskID == 0 || s.eventHub == nil {
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s 失败: %w, output=%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return nil
	}

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
			s.publish(taskID, taskEventTypeLog, "git_output", model.TaskStatusRunning, line, map[string]any{
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

func (s *RuntimeRepoSupport) publish(taskID uint64, eventType, stage, status, message string, data map[string]any) {
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
