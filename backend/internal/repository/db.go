package repository

import (
	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.DatabaseConfig, zapLogger *zap.Logger) error {
	logLevel := logger.Warn
	if cfg.Host == "" {
		logLevel = logger.Silent
	}
	if config.Global != nil {
		if strings.EqualFold(config.Global.Log.Level, "debug") || strings.EqualFold(config.Global.Server.Mode, "debug") {
			logLevel = logger.Info
		}
	}

	gormLogger := logger.New(zap.NewStdLog(zapLogger), logger.Config{
		SlowThreshold:             500 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
		LogLevel:                  logLevel,
		Colorful:                  false,
	})

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)

	DB = db
	if err := ensureRuntimeTables(db); err != nil {
		return err
	}
	zapLogger.Info("数据库连接成功", zap.String("host", cfg.Host), zap.String("db", cfg.DBName))
	return nil
}

func ensureRuntimeTables(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.IssueSyncLogDetail{}) {
		if err := db.AutoMigrate(&model.IssueSyncLogDetail{}); err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(&model.APIExchangeLog{}) {
		if err := db.AutoMigrate(&model.APIExchangeLog{}); err != nil {
			return err
		}
	}
	return nil
}
