package cron

import (
	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron          *cron.Cron
	logger        *zap.Logger
	zentaoService *service.ZentaoService
}

// NewScheduler 创建调度器
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:          cron.New(cron.WithSeconds()),
		logger:        logger,
		zentaoService: service.NewZentaoService(logger),
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
		s.logger.Info("定时任务: 开始同步禅道问题单")
		s.zentaoService.SyncAllProjects()
	})
	if err != nil {
		s.logger.Error("注册禅道同步定时任务失败", zap.Error(err))
	} else {
		s.logger.Info("禅道同步定时任务已注册", zap.String("cron", cronExpr))
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

// autoTriggerGenTest 自动为"已解决"且"未测试"的问题单触发AI生成
func (s *Scheduler) autoTriggerGenTest() {
	projectRepo := repository.NewProjectRepo()
	issueRepo := repository.NewIssueRepo()

	projects, err := projectRepo.GetAllActive()
	if err != nil {
		s.logger.Error("获取项目列表失败", zap.Error(err))
		return
	}

	for _, p := range projects {
		// 查找已解决且测试状态为pending的问题单
		issues, err := issueRepo.GetResolvedPending(p.ID)
		if err != nil {
			s.logger.Error("查询问题单失败", zap.Uint64("project_id", p.ID), zap.Error(err))
			continue
		}

		for _, issue := range issues {
			if issue.TestStatus != model.TestStatusPending {
				continue
			}
			s.logger.Info("自动触发AI生成",
				zap.Uint64("issue_id", issue.ID),
				zap.String("title", issue.Title))

			_, err := service.NewGenTestService(s.logger).Execute(issue.ID, nil, nil, "")
			if err != nil {
				s.logger.Error("自动生成失败", zap.Uint64("issue_id", issue.ID), zap.Error(err))
			}
		}
	}
}
