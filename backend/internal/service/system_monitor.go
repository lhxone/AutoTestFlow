package service

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"auto-test-flow/internal/config"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"
)

type WorkspaceMetrics struct {
	ProjectID     uint64 `json:"project_id"`
	ProjectName   string `json:"project_name"`
	WorkspaceSize int64  `json:"workspace_size"`
}

type DiskInfo struct {
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	UsedPercentage float64 `json:"used_percentage"`
}

type SystemMetrics struct {
	Disk             DiskInfo           `json:"disk"`
	CPUPercentage    float64            `json:"cpu_percentage"`
	MemoryTotalBytes uint64             `json:"memory_total_bytes"`
	MemoryUsedBytes  uint64             `json:"memory_used_bytes"`
	MemoryPercentage float64            `json:"memory_percentage"`
	Workspaces       []WorkspaceMetrics `json:"workspaces"`
}

type SystemMonitorService struct{}

func NewSystemMonitorService() *SystemMonitorService {
	return &SystemMonitorService{}
}

func (s *SystemMonitorService) GetSystemMetrics() (*SystemMetrics, error) {
	result := &SystemMetrics{}

	diskInfo, err := getDiskInfo()
	if err == nil {
		result.Disk = diskInfo
	}

	memInfo, err := getMemoryInfo()
	if err == nil {
		result.MemoryTotalBytes = memInfo.TotalBytes
		result.MemoryUsedBytes = memInfo.UsedBytes
		result.MemoryPercentage = memInfo.Percentage
	}

	result.CPUPercentage = getCPUPercentage()

	workspaces, err := s.collectWorkspaceSizes()
	if err == nil {
		result.Workspaces = workspaces
	}

	return result, nil
}

type memoryInfo struct {
	TotalBytes uint64
	UsedBytes  uint64
	Percentage float64
}

func getWorkspaceRoot() string {
	workspaceRoot := config.Global.CLIRuntime.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(config.Global.Git.WorkDir, "cli-runtime")
	}
	return workspaceRoot
}

func (s *SystemMonitorService) collectWorkspaceSizes() ([]WorkspaceMetrics, error) {
	projects, err := repository.NewProjectRepo().GetAllActive()
	if err != nil {
		return nil, err
	}

	workspaceRoot := getWorkspaceRoot()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []WorkspaceMetrics
	)

	for _, p := range projects {
		wg.Add(1)
		go func(proj model.Project) {
			defer wg.Done()
			projectDir := filepath.Join(workspaceRoot, "project_"+strconv.FormatUint(uint64(proj.ID), 10))
			size := dirSize(projectDir)
			mu.Lock()
			results = append(results, WorkspaceMetrics{
				ProjectID:     proj.ID,
				ProjectName:   proj.Name,
				WorkspaceSize: size,
			})
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	return results, nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
