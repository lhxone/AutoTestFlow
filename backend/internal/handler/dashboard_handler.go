package handler

import (
	"time"

	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct{}

type DashboardStats struct {
	Projects            *int64                       `json:"projects"`
	PendingReviews      *int64                       `json:"pending_reviews"`
	InterventionNeeded  *int64                       `json:"intervention_needed"`
	PassRate            *float64                     `json:"pass_rate"`
	IssueSyncProjects   []DashboardProjectSyncStatus `json:"issue_sync_projects,omitempty"`
	TestCaseSyncProjects []DashboardProjectSyncStatus `json:"testcase_sync_projects,omitempty"`
}

type DashboardProjectSyncStatus struct {
	ProjectID    uint64  `json:"project_id"`
	ProjectName  string  `json:"project_name"`
	Status       string  `json:"status"`
	StatusLabel  string  `json:"status_label"`
	AddedCount   int     `json:"added_count"`
	UpdatedCount int     `json:"updated_count"`
	DeletedCount int     `json:"deleted_count"`
	StartedAt    *string `json:"started_at,omitempty"`
	CompletedAt  *string `json:"completed_at,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// GetStats Dashboard 统计数据
// GET /api/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	roleCode := middleware.GetCurrentRoleCode(c)
	stats := DashboardStats{}

	if canAccess(roleCode, "project:list") {
		var count int64
		if err := repository.DB.Model(&model.Project{}).Count(&count).Error; err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}
		stats.Projects = &count
	}

	if canAccess(roleCode, "review:list") {
		var count int64
		if err := repository.DB.Model(&model.ReviewTask{}).
			Where("status = ?", model.ReviewStatusPending).
			Count(&count).Error; err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}
		stats.PendingReviews = &count
	}

	if canAccess(roleCode, "issue:list") {
		var count int64
		if err := repository.DB.Model(&model.Issue{}).
			Where("test_status = ?", model.TestStatusInterventionNeeded).
			Count(&count).Error; err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}
		stats.InterventionNeeded = &count
	}

	if canAccess(roleCode, "review:list") {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var todayTotal int64
		if err := repository.DB.Model(&model.ReviewTask{}).
			Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
			Count(&todayTotal).Error; err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}

		var todayApproved int64
		if err := repository.DB.Model(&model.ReviewTask{}).
			Where("status = ? AND created_at >= ? AND created_at < ?", model.ReviewStatusApproved, startOfDay, endOfDay).
			Count(&todayApproved).Error; err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}

		passRate := 0.0
		if todayTotal > 0 {
			passRate = float64(todayApproved) / float64(todayTotal) * 100
		}
		stats.PassRate = &passRate
	}

	if canAccess(roleCode, "issue:list") || canAccess(roleCode, "issue:sync") || canAccess(roleCode, "project:list") {
		projects, err := repository.NewProjectRepo().GetAllActive()
		if err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}

		projectIDs := make([]uint64, 0, len(projects))
		for _, project := range projects {
			projectIDs = append(projectIDs, project.ID)
		}

		latestLogs, err := repository.NewIssueSyncLogRepo().GetLatestByProjectIDs(projectIDs)
		if err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}

		logMap := make(map[uint64]model.IssueSyncLog, len(latestLogs))
		for _, log := range latestLogs {
			logMap[log.ProjectID] = log
		}

		stats.IssueSyncProjects = make([]DashboardProjectSyncStatus, 0, len(projects))
		for _, project := range projects {
			item := DashboardProjectSyncStatus{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				Status:      "unknown",
				StatusLabel: "未同步",
			}

			if log, ok := logMap[project.ID]; ok {
				item.Status = log.Status
				item.StatusLabel = syncStatusLabel(log.Status)
				item.AddedCount = log.AddedCount
				item.UpdatedCount = log.UpdatedCount
				item.DeletedCount = log.DeletedCount
				item.ErrorMessage = log.ErrorMessage
				startedAt := log.StartedAt.Format("2006-01-02 15:04:05")
				item.StartedAt = &startedAt
				if log.CompletedAt != nil {
					completedAt := log.CompletedAt.Format("2006-01-02 15:04:05")
					item.CompletedAt = &completedAt
				}
			}

			stats.IssueSyncProjects = append(stats.IssueSyncProjects, item)
		}

		// 用例同步状态
		latestTestCaseLogs, err := repository.NewIssueSyncLogRepo().GetLatestByProjectIDsAndType(projectIDs, model.SyncTypeTestCase)
		if err != nil {
			pkg.Fail(c, pkg.CodeInternalError, err.Error())
			return
		}

		testCaseLogMap := make(map[uint64]model.IssueSyncLog, len(latestTestCaseLogs))
		for _, log := range latestTestCaseLogs {
			testCaseLogMap[log.ProjectID] = log
		}

		stats.TestCaseSyncProjects = make([]DashboardProjectSyncStatus, 0, len(projects))
		for _, project := range projects {
			item := DashboardProjectSyncStatus{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				Status:      "unknown",
				StatusLabel: "未同步",
			}

			if log, ok := testCaseLogMap[project.ID]; ok {
				item.Status = log.Status
				item.StatusLabel = syncStatusLabel(log.Status)
				item.AddedCount = log.AddedCount
				item.UpdatedCount = log.UpdatedCount
				item.DeletedCount = log.DeletedCount
				item.ErrorMessage = log.ErrorMessage
				startedAt := log.StartedAt.Format("2006-01-02 15:04:05")
				item.StartedAt = &startedAt
				if log.CompletedAt != nil {
					completedAt := log.CompletedAt.Format("2006-01-02 15:04:05")
					item.CompletedAt = &completedAt
				}
			}

			stats.TestCaseSyncProjects = append(stats.TestCaseSyncProjects, item)
		}
	}

	pkg.OK(c, stats)
}

func canAccess(roleCode, permCode string) bool {
	if roleCode == "admin" {
		return true
	}

	return middleware.HasPermission(roleCode, permCode)
}

func syncStatusLabel(status string) string {
	switch status {
	case model.IssueSyncStatusSuccess:
		return "正常"
	case model.IssueSyncStatusFailed:
		return "异常"
	case model.IssueSyncStatusRunning:
		return "进行中"
	default:
		return "未同步"
	}
}

type RecentActivity struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	ActionLabel string  `json:"action_label"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *DashboardHandler) GetRecentActivities(c *gin.Context) {
	var logs []model.OperationLog
	if err := repository.DB.Model(&model.OperationLog{}).
		Where("module = ?", "auth").
		Order("id DESC").
		Limit(10).
		Find(&logs).Error; err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	activities := make([]RecentActivity, 0, len(logs))
	for _, log := range logs {
		actionLabel := "未知操作"
		switch log.Action {
		case "login_success":
			actionLabel = "登录成功"
		case "login_failed":
			actionLabel = "登录失败"
		case "logout":
			actionLabel = "退出登录"
		}
		activities = append(activities, RecentActivity{
			ID:          log.ID,
			Username:    log.Username,
			Action:      log.Action,
			ActionLabel: actionLabel,
			IP:          log.IP,
			CreatedAt:   log.CreatedAt,
		})
	}

	pkg.OK(c, activities)
}

// GetMonitorKPI returns system monitoring metrics
// GET /api/dashboard/monitor
func (h *DashboardHandler) GetMonitorKPI(c *gin.Context) {
	monitorSvc := service.NewSystemMonitorService()
	metrics, err := monitorSvc.GetSystemMetrics()
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, metrics)
}
