package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const gitCommandTimeout = 20 * time.Second

type GitOpsService struct {
	testTaskRepo  *repository.TestTaskRepo
	projectRepo   *repository.ProjectRepo
	executionRepo *repository.ExecutionRepo
	settingRepo   *repository.SettingRepo
	logger        *zap.Logger
}

func NewGitOpsService(logger *zap.Logger) *GitOpsService {
	return &GitOpsService{
		testTaskRepo:  repository.NewTestTaskRepo(),
		projectRepo:   repository.NewProjectRepo(),
		executionRepo: repository.NewExecutionRepo(),
		settingRepo:   repository.NewSettingRepo(),
		logger:        logger,
	}
}

// PushReviewedContent 将审核通过的内容推送到Git仓库
func (s *GitOpsService) PushReviewedContent(reviewTask *model.ReviewTask) error {
	project, err := s.projectRepo.GetByID(reviewTask.ProjectID)
	if err != nil {
		return fmt.Errorf("项目不存在: %w", err)
	}

	if project.GitRepoURL == "" {
		return fmt.Errorf("项目未配置Git仓库")
	}

	// 获取测试内容
	scripts, _ := s.testTaskRepo.GetTestScriptsByTaskID(reviewTask.TestTaskID)
	docs, _ := s.testTaskRepo.GetTestDocsByTaskID(reviewTask.TestTaskID)

	cfg := config.Global.Git
	repoDir := filepath.Join(cfg.WorkDir, fmt.Sprintf("project_%d", project.ID))

	// 确保仓库已clone
	if err := s.ensureRepo(repoDir, project.GitRepoURL, project.GitBranch); err != nil {
		return fmt.Errorf("Git仓库准备失败: %w", err)
	}
	_ = s.ensureGitIdentity(repoDir)

	// 获取 ZentaoID 用于分支命名
	var zentaoID int
	if reviewTask.Issue != nil {
		zentaoID = reviewTask.Issue.ZentaoID
	}

	// 创建 feature 分支
	var branchName string
	if zentaoID > 0 {
		branchName = fmt.Sprintf("autotest/review-zentao-%d", zentaoID)
	} else {
		branchName = fmt.Sprintf("autotest/review-%d", reviewTask.ID)
	}
	if err := s.gitExec(repoDir, "checkout", "-b", branchName); err != nil {
		// 分支可能已存在
		_ = s.gitExec(repoDir, "checkout", branchName)
	}

	// 写入测试脚本文件
	for _, script := range scripts {
		filePath := filepath.Join(repoDir, script.FilePath)
		dir := filepath.Dir(filePath)
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filePath, []byte(script.FileContent), 0644); err != nil {
			s.logger.Error("写入脚本文件失败", zap.String("path", filePath), zap.Error(err))
			continue
		}
	}

	// 写入测试文档
	for _, doc := range docs {
		if doc.FilePath == "" {
			doc.FilePath = fmt.Sprintf("docs/test_%d.md", doc.ID)
		}
		filePath := filepath.Join(repoDir, doc.FilePath)
		dir := filepath.Dir(filePath)
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filePath, []byte(doc.Content), 0644); err != nil {
			s.logger.Error("写入文档文件失败", zap.String("path", filePath), zap.Error(err))
			continue
		}
	}

	// Git add + commit + push
	_ = s.gitExec(repoDir, "add", ".")

	filesChanged, _ := s.gitOutput(repoDir, "diff", "--cached", "--name-only")
	changedFiles := splitLines(filesChanged)
	if len(changedFiles) == 0 {
		s.logger.Info("审核内容无文件变更，跳过Git提交", zap.Uint64("review_id", reviewTask.ID))
		filesJSON, _ := json.Marshal(changedFiles)
		_ = s.executionRepo.CreateGitCommitLog(&model.GitCommitLog{
			ReviewTaskID:  &reviewTask.ID,
			TestTaskID:    &reviewTask.TestTaskID,
			ProjectID:     project.ID,
			Branch:        branchName,
			CommitMessage: "无文件变更，跳过提交",
			FilesChanged:  model.JSON(filesJSON),
			PushStatus:    "skipped",
		})
		return nil
	}

	commitMsg := fmt.Sprintf("[AutoTestFlow] Review #%d 通过 - %s", reviewTask.ID, reviewTask.Title)
	if err := s.gitExec(repoDir, "commit", "-m", commitMsg,
		"--author", fmt.Sprintf("%s <%s>", cfg.CommitAuthor, cfg.CommitEmail)); err != nil {
		return fmt.Errorf("Git commit失败: %w", err)
	}

	// 获取commit hash
	hash, _ := s.gitOutput(repoDir, "rev-parse", "HEAD")

	if err := s.gitExec(repoDir, "push", "origin", branchName); err != nil {
		// 记录push失败
		detail, _ := json.Marshal(map[string]string{"error": err.Error()})
		s.executionRepo.CreateOperationLog(&model.OperationLog{
			Module:     "gitops",
			Action:     "push_failed",
			TargetType: "review_task",
			TargetID:   &reviewTask.ID,
			Detail:     model.JSON(detail),
		})
		return fmt.Errorf("Git push失败: %w", err)
	}

	// 记录提交日志
	filesJSON, _ := json.Marshal(changedFiles)
	commitLog := &model.GitCommitLog{
		ReviewTaskID:  &reviewTask.ID,
		TestTaskID:    &reviewTask.TestTaskID,
		ProjectID:     project.ID,
		CommitHash:    strings.TrimSpace(hash),
		Branch:        branchName,
		CommitMessage: commitMsg,
		FilesChanged:  model.JSON(filesJSON),
		PushStatus:    "success",
	}
	_ = s.executionRepo.CreateGitCommitLog(commitLog)

	s.logger.Info("Git推送成功",
		zap.Uint64("review_id", reviewTask.ID),
		zap.String("branch", branchName),
		zap.String("commit", strings.TrimSpace(hash)))

	// 创建 Merge Request
	projectPath := s.parseGitLabProjectPath(project.GitRepoURL)
	mrTitle := fmt.Sprintf("[AutoTestFlow] Review #%d - %s", reviewTask.ID, reviewTask.Title)
	if err := s.CreateMergeRequest(projectPath, branchName, project.GitBranch, mrTitle); err != nil {
		s.logger.Warn("创建 MergeRequest 失败", zap.Error(err), zap.String("branch", branchName))
		// 不阻断流程，仅记录日志
	}

	return nil
}

// ensureRepo 确保Git仓库已clone且是最新
func (s *GitOpsService) ensureRepo(repoDir, repoURL, branch string) error {
	authURL := s.withGitCredentials(repoURL)
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		return s.cloneRepo(repoDir, authURL, branch)
	}

	healthy, err := s.isRepoHealthy(repoDir)
	if err != nil {
		return err
	}
	if !healthy {
		s.logger.Warn("检测到损坏的Git工作区，准备重新克隆", zap.String("repo_dir", repoDir))
		if removeErr := os.RemoveAll(repoDir); removeErr != nil {
			return fmt.Errorf("本地Git仓库状态异常，清理失败: %w", removeErr)
		}
		return s.cloneRepo(repoDir, authURL, branch)
	}

	if err := s.gitExec(repoDir, "remote", "set-url", "origin", authURL); err != nil {
		return err
	}
	if err := s.gitExec(repoDir, "fetch", "--prune", "origin", branch); err != nil {
		return err
	}
	if err := s.gitExec(repoDir, "checkout", branch); err != nil {
		return err
	}
	if err := s.gitExec(repoDir, "reset", "--hard", "origin/"+branch); err != nil {
		return err
	}

	return nil
}

func (s *GitOpsService) cloneRepo(repoDir, repoURL, branch string) error {
	_ = os.MkdirAll(filepath.Dir(repoDir), 0755)
	if err := s.gitExec("", "clone", "-b", branch, repoURL, repoDir); err != nil {
		return fmt.Errorf("git clone失败: %w", err)
	}
	return nil
}

func (s *GitOpsService) isRepoHealthy(repoDir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := s.gitOutput(repoDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return false, nil
	}
	if _, err := s.gitOutput(repoDir, "symbolic-ref", "-q", "HEAD"); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *GitOpsService) ensureGitIdentity(repoDir string) error {
	cfg := config.Global.Git
	if err := s.gitExec(repoDir, "config", "user.name", cfg.CommitAuthor); err != nil {
		return err
	}
	if err := s.gitExec(repoDir, "config", "user.email", cfg.CommitEmail); err != nil {
		return err
	}
	return nil
}

func (s *GitOpsService) withGitCredentials(repoURL string) string {
	token := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "access_token"))
	baseURL := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "base_url"))
	if token == "" || baseURL == "" {
		return repoURL
	}

	repoParsed, err := url.Parse(repoURL)
	if err != nil || repoParsed.Scheme == "" || repoParsed.Host == "" {
		return repoURL
	}

	baseParsed, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(repoParsed.Host, baseParsed.Host) {
		return repoURL
	}

	repoParsed.User = url.UserPassword("oauth2", token)
	return repoParsed.String()
}

func splitLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func (s *GitOpsService) gitExec(dir string, args ...string) error {
	ctx, cmd, cancel := s.gitCommand(dir, args...)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.logger.Error("Git命令超时", zap.Strings("args", args), zap.String("output", string(output)))
			return fmt.Errorf("git命令超时: %s", strings.TrimSpace(string(output)))
		}
		s.logger.Error("Git命令失败",
			zap.Strings("args", args),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}

func (s *GitOpsService) gitOutput(dir string, args ...string) (string, error) {
	ctx, cmd, cancel := s.gitCommand(dir, args...)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(output), fmt.Errorf("git命令超时: %w", err)
	}
	return string(output), err
}

func (s *GitOpsService) gitCommand(dir string, args ...string) (context.Context, *exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
		"GIT_ASKPASS=",
	)
	return ctx, cmd, cancel
}

// parseGitLabProjectPath 从 Git URL 解析项目路径
// http://git.inspur.com/kms2/kms-project-test.git → kms2%2Fkms-project-test
func (s *GitOpsService) parseGitLabProjectPath(gitURL string) string {
	u, err := url.Parse(gitURL)
	if err != nil {
		return ""
	}

	// 获取路径部分，去掉 .git 后缀
	path := strings.TrimSuffix(u.Path, ".git")
	path = strings.TrimPrefix(path, "/")

	// URL encode (GitLab API 要求 / 编码为 %2F)
	return url.PathEscape(path)
}

// CreateMergeRequest 调用 GitLab API 创建 Merge Request
func (s *GitOpsService) CreateMergeRequest(projectPath, sourceBranch, targetBranch, title string) error {
	baseURL := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "base_url"))
	accessToken := strings.TrimSpace(s.settingRepo.GetValue("gitlab", "access_token"))

	if baseURL == "" || accessToken == "" {
		return errors.New("GitLab 配置缺失")
	}

	// POST /api/v4/projects/{id}/merge_requests
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", strings.TrimRight(baseURL, "/"), projectPath)

	payload := map[string]interface{}{
		"source_branch":        sourceBranch,
		"target_branch":        targetBranch,
		"title":                title,
		"remove_source_branch": true,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitLab API 错误: %s", string(respBody))
	}

	return nil
}
