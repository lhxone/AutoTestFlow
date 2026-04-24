package cron

import (
	"context"
	"sync/atomic"
	"time"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron            *cron.Cron
	logger          *zap.Logger
	zentaoService   *service.ZentaoService
	zentaoTCService *service.ZentaoTestCaseSyncService
	gitPullService  *service.GitPullService
	gitPullRunning  atomic.Bool
}

// NewScheduler 创建调度器
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:            cron.New(cron.WithSeconds()),
		logger:          logger,
		zentaoService:   service.NewZentaoService(logger),
		zentaoTCService: service.NewZentaoTestCaseSyncService(logger),
		gitPullService:  service.NewGitPullService(logger),
	}
}

// Start 启动所有定时任务
func (s *Scheduler) Start() {
	cfg := config.Global.Zentao

	// 1. 禅道问题单定时同步
	cronExpr := cfg.SyncInterval
	if cronExpr == "" {
		cronExpr = "0 */30 * * * *" // 默认每30分钟(带秒)
	} else {
		// 转换5段cron到6段(加秒前缀)
		cronExpr = "0 " + cronExpr
	}

	_, err := s.cron.AddFunc(cronExpr, func() {
		s.logger.Info("定时任务: 开始全量同步禅道问题单")
		s.zentaoService.SyncAllProjects(true)
	})
	if err != nil {
		s.logger.Error("注册禅道同步定时任务失败", zap.Error(err))
	} else {
		s.logger.Info("禅道同步定时任务已注册", zap.String("cron", cronExpr))
	}

	// 1.5 禅道用例定时同步（复用相同的 cron 表达式）
	_, err = s.cron.AddFunc(cronExpr, func() {
		s.logger.Info("定时任务: 开始同步禅道用例")
		s.syncAllTestCases()
	})
	if err != nil {
		s.logger.Error("注册禅道用例同步定时任务失败", zap.Error(err))
	} else {
		s.logger.Info("禅道用例同步定时任务已注册", zap.String("cron", cronExpr))
	}

	// 2. 项目共享仓库定时拉取（每分钟巡检一次，按项目配置频率触发）
	_, err = s.cron.AddFunc("0 * * * * *", func() {
		if !s.gitPullRunning.CompareAndSwap(false, true) {
			s.logger.Warn("项目 Git 定时拉取仍在执行，跳过本轮")
			return
		}
		defer s.gitPullRunning.Store(false)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		s.logger.Info("定时任务: 开始执行项目共享仓库拉取巡检")
		s.gitPullService.SyncDueProjects(ctx)
	})
	if err != nil {
		s.logger.Error("注册项目 Git 拉取定时任务失败", zap.Error(err))
	} else {
		s.logger.Info("项目 Git 拉取定时任务已注册", zap.String("cron", "0 * * * * *"))
	}

	// 自动触发生成已暂停，仅保留手动触发入口
	// 如需启用，取消下方注释
	// _, err = s.cron.AddFunc("0 0 * * * *", func() {
	// 	s.logger.Info("定时任务: 检查已解决问题单并触发AI生成")
	// 	s.autoTriggerGenTest()
	// })
	// if err != nil {
	// 	s.logger.Error("注册自动生成定时任务失败", zap.Error(err))
	// }

	s.cron.Start()
	s.logger.Info("定时任务调度器已启动")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.logger.Info("定时任务调度器已停止")
}

// syncAllTestCases 同步所有项目的禅道用例
func (s *Scheduler) syncAllTestCases() {
	projectRepo := repository.NewProjectRepo()

	projects, err := projectRepo.GetAllActive()
	if err != nil {
		s.logger.Error("获取项目列表失败", zap.Error(err))
		return
	}

	for _, p := range projects {
		if p.ZentaoProjectID == nil {
			continue
		}
		result, err := s.zentaoTCService.SyncTestCases(p.ID, false)
		if err != nil {
			s.logger.Error("同步用例失败", zap.Uint64("project_id", p.ID), zap.Error(err))
			continue
		}
		s.logger.Info("用例同步完成",
			zap.String("project", p.Name),
			zap.Int("synced", result.SyncedCount),
			zap.Int("added", result.AddedCount),
			zap.Int("updated", result.UpdatedCount),
			zap.Int("deleted", result.DeletedCount))
	}
}
