package service

import (
	"context"
	"sync"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

// PendingGenerateService 统一消费待生成工单，并按运行时配置控制并发与超时。
type PendingGenerateService struct {
	issueRepo      *repository.IssueRepo
	genTestService *GenTestService
	logger         *zap.Logger
}

func NewPendingGenerateService(logger *zap.Logger) *PendingGenerateService {
	return &PendingGenerateService{
		issueRepo:      repository.NewIssueRepo(),
		genTestService: NewGenTestService(logger),
		logger:         logger,
	}
}

func (s *PendingGenerateService) ProcessPendingGenerate(ctx context.Context) error {
	issues, err := s.issueRepo.FindPendingGenerate()
	if err != nil {
		return err
	}
	if len(issues) == 0 {
		s.logger.Debug("待生成问题单为空，跳过本轮调度")
		return nil
	}

	s.logger.Info("发现待生成问题单，准备触发生成任务", zap.Int("issue_count", len(issues)))
	s.DispatchIssues(ctx, issues)
	return nil
}

func (s *PendingGenerateService) DispatchIssues(ctx context.Context, issues []model.Issue) {
	if len(issues) == 0 {
		return
	}

	settings := LoadRuntimeSettings()
	workerCount := settings.MaxConcurrentTasks
	if workerCount <= 0 {
		workerCount = defaultMaxConcurrentTasks
	}
	if workerCount > len(issues) {
		workerCount = len(issues)
	}

	jobCh := make(chan model.Issue)
	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for issue := range jobCh {
				s.dispatchIssue(ctx, issue, settings)
			}
		}()
	}

	for _, issue := range issues {
		select {
		case <-ctx.Done():
			close(jobCh)
			wg.Wait()
			return
		case jobCh <- issue:
		}
	}

	close(jobCh)
	wg.Wait()
	s.logger.Info("待生成问题单调度完成",
		zap.Int("total", len(issues)),
		zap.Int("max_concurrent", settings.MaxConcurrentTasks))
}

func (s *PendingGenerateService) dispatchIssue(parent context.Context, issue model.Issue, settings RuntimeSettings) {
	claimed, err := s.issueRepo.TryClaimPendingGenerate(issue.ID)
	if err != nil {
		s.logger.Error("抢占待生成问题单失败", zap.Uint64("issue_id", issue.ID), zap.Error(err))
		return
	}
	if !claimed {
		s.logger.Debug("待生成问题单已被其他调度处理", zap.Uint64("issue_id", issue.ID))
		return
	}

	task, err := s.genTestService.CreatePendingTask(issue.ID, nil, nil, "")
	if err != nil {
		s.logger.Error("创建待生成问题单测试任务失败", zap.Uint64("issue_id", issue.ID), zap.Error(err))
		_ = s.issueRepo.ForceUpdateTestStatus(issue.ID, model.TestStatusError)
		return
	}

	taskCtx, cancel := context.WithTimeout(parent, settings.TaskTimeout)
	defer cancel()

	s.logger.Info("开始执行待生成问题单测试任务",
		zap.Uint64("issue_id", issue.ID),
		zap.Uint64("task_id", task.ID),
		zap.Duration("timeout", settings.TaskTimeout))

	if err := s.genTestService.RunTask(taskCtx, task.ID); err != nil {
		s.logger.Error("待生成问题单测试任务执行失败",
			zap.Uint64("issue_id", issue.ID),
			zap.Uint64("task_id", task.ID),
			zap.Error(err))
		return
	}
}
