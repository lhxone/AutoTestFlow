package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"auto-test-flow/internal/config"
	appCron "auto-test-flow/internal/cron"
	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/router"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	// 1. 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	logger := initLogger(cfg.Log.Level)
	defer logger.Sync()

	logger.Info("AutoTestFlow 启动中...",
		zap.String("mode", cfg.Server.Mode),
		zap.Int("port", cfg.Server.Port))

	// 3. 设置 Gin 模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 4. 初始化数据库
	if err := repository.InitDB(&cfg.Database, logger); err != nil {
		logger.Fatal("初始化数据库失败", zap.Error(err))
	}

	// 5. 加载权限缓存
	loadPermissionCache(repository.DB, logger)

	// 5.5 初始化事件持久化
	service.DefaultTaskEventHub.SetRepo(repository.NewTaskEventLogRepo())

	// 6. 初始化路由
	r := router.Setup(logger)

	// 7. 启动定时任务
	scheduler := appCron.NewScheduler(logger)
	scheduler.Start()

	// 8. 启动HTTP服务(非阻塞)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	go func() {
		logger.Info("HTTP服务启动", zap.String("addr", addr))
		if err := r.Run(addr); err != nil {
			logger.Fatal("HTTP服务启动失败", zap.Error(err))
		}
	}()

	// 9. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("收到退出信号", zap.String("signal", sig.String()))

	scheduler.Stop()
	logger.Info("AutoTestFlow 已停止")
}

// loadPermissionCache 从数据库加载角色权限到内存缓存
func loadPermissionCache(db *gorm.DB, logger *zap.Logger) {
	type RolePerm struct {
		RoleCode string
		PermCode string
	}

	var results []RolePerm
	err := db.Raw(`
		SELECT r.code AS role_code, p.code AS perm_code
		FROM role_permission rp
		JOIN role r ON r.id = rp.role_id
		JOIN permission p ON p.id = rp.permission_id
	`).Scan(&results).Error

	if err != nil {
		logger.Error("加载权限缓存失败", zap.Error(err))
		return
	}

	cache := make(map[string]map[string]bool)
	for _, rp := range results {
		if cache[rp.RoleCode] == nil {
			cache[rp.RoleCode] = make(map[string]bool)
		}
		cache[rp.RoleCode][rp.PermCode] = true
	}

	middleware.InitPermissionCache(cache)
	logger.Info("权限缓存加载完成", zap.Int("roles", len(cache)))
}

func initLogger(level string) *zap.Logger {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zap.DebugLevel
	case "warn":
		zapLevel = zap.WarnLevel
	case "error":
		zapLevel = zap.ErrorLevel
	default:
		zapLevel = zap.InfoLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      zapLevel == zap.DebugLevel,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	return logger
}
