package service

import (
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/repository"
)

const (
	defaultMaxConcurrentTasks             = 1
	defaultTaskTimeoutMinutes             = 30
	defaultPendingGenerateIntervalMinutes = 1
)

type RuntimeSettings struct {
	MaxConcurrentTasks             int
	TaskTimeout                    time.Duration
	PendingGenerateIntervalMinutes int
}

func LoadRuntimeSettings() RuntimeSettings {
	repo := repository.NewSettingRepo()

	maxConcurrent := positiveInt(repo.GetValue("runtime", "max_concurrent_tasks"), 0)
	if maxConcurrent <= 0 {
		maxConcurrent = positiveInt(repo.GetValue("integration", "max_concurrent_tasks"), defaultMaxConcurrentTasks)
	}

	timeoutMinutes := positiveInt(repo.GetValue("runtime", "task_timeout_minutes"), defaultTaskTimeoutMinutes)
	intervalMinutes := positiveInt(repo.GetValue("runtime", "pending_generate_interval_minutes"), defaultPendingGenerateIntervalMinutes)

	return RuntimeSettings{
		MaxConcurrentTasks:             maxConcurrent,
		TaskTimeout:                    time.Duration(timeoutMinutes) * time.Minute,
		PendingGenerateIntervalMinutes: intervalMinutes,
	}
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
