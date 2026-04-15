package repository

import (
	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.DatabaseConfig, zapLogger *zap.Logger) error {
	logLevel := logger.Info
	if cfg.Host == "" {
		logLevel = logger.Silent
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
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
	return nil
}
