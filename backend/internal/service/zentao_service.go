package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type ZentaoService struct {
	issueRepo   *repository.IssueRepo
	syncLogRepo *repository.IssueSyncLogRepo
	projectRepo *repository.ProjectRepo
	settingRepo *repository.SettingRepo
	settingSvc  *SettingService
	logger      *zap.Logger
	httpClient  *http.Client
}

func NewZentaoService(logger *zap.Logger) *ZentaoService {
	return &ZentaoService{
		issueRepo:   repository.NewIssueRepo(),
		syncLogRepo: repository.NewIssueSyncLogRepo(),
		projectRepo: repository.NewProjectRepo(),
		settingRepo: repository.NewSettingRepo(),
		settingSvc:  NewSettingService(logger),
		logger:      logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

type issueSyncResult struct {
	SyncedCount  int
	AddedCount   int
	UpdatedCount int
	DeletedCount int
	Details      []model.IssueSyncLogDetail
}

type zentaoUserRef struct {
	Account  string `json:"account"`
	Realname string `json:"realname"`
	Email    string `json:"email"`
}

type zentaoBug struct {
	ID             int            `json:"id"`
	Branch         int            `json:"branch"`
	Title          string         `json:"title"`
	Steps          string         `json:"steps"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	Severity       int            `json:"severity"`
	Pri            int            `json:"pri"`
	OpenedBy       *zentaoUserRef `json:"openedBy"`
	AssignedTo     *zentaoUserRef `json:"assignedTo"`
	ResolvedDate   *string        `json:"resolvedDate"`
	LastEditedDate *string        `json:"lastEditedDate"`
}

// StartAsyncSync 异步触发同步，立即返回 syncLogID
func (s *ZentaoService) StartAsyncSync(projectID uint64, fullSync bool) uint64 {
	syncLog := &model.IssueSyncLog{
		ProjectID: projectID,
		Status:    model.IssueSyncStatusRunning,
		FullSync:  fullSync,
	}
	_ = s.syncLogRepo.Create(syncLog)

	go s.executeSync(projectID, fullSync, syncLog.ID)

	return syncLog.ID
}

// executeSync 执行同步逻辑（内部方法）
func (s *ZentaoService) executeSync(projectID uint64, fullSync bool, syncLogID uint64) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, issueSyncResult{}, fmt.Errorf("项目不存在: %w", err))
		return
	}

	if project.ZentaoProjectID == nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, issueSyncResult{}, fmt.Errorf("项目未配置禅道项目ID"))
		return
	}

	baseURL, token, err := s.getZentaoConnection()
	if err != nil {
		s.logger.Warn("禅道未配置，使用Mock数据同步", zap.Error(err))
		result, syncErr := s.syncWithMockData(project)
		s.persistSyncDetails(syncLogID, result.Details)
		if syncErr != nil {
			s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, result, syncErr)
			return
		}
		s.finishSyncLog(syncLogID, model.IssueSyncStatusSuccess, result, nil)
		return
	}

	result, err := s.syncFromZentao(project, fullSync, baseURL, token)
	s.persistSyncDetails(syncLogID, result.Details)
	if err != nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, result, err)
		return
	}

	s.finishSyncLog(syncLogID, model.IssueSyncStatusSuccess, result, nil)
}

// SyncProjectIssues 同步指定项目的问题单（同步版本，保留用于定时任务等场景）
func (s *ZentaoService) SyncProjectIssues(projectID uint64, fullSync bool) (int, error) {
	syncLog := &model.IssueSyncLog{
		ProjectID: projectID,
		Status:    model.IssueSyncStatusRunning,
		FullSync:  fullSync,
	}
	_ = s.syncLogRepo.Create(syncLog)

	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		s.finishSyncLog(syncLog.ID, model.IssueSyncStatusFailed, issueSyncResult{}, err)
		return 0, fmt.Errorf("项目不存在: %w", err)
	}

	if project.ZentaoProjectID == nil {
		s.finishSyncLog(syncLog.ID, model.IssueSyncStatusFailed, issueSyncResult{}, fmt.Errorf("项目未配置禅道项目ID"))
		return 0, fmt.Errorf("项目未配置禅道项目ID")
	}

	baseURL, token, err := s.getZentaoConnection()
	if err != nil {
		// 保留旧的 Mock 兜底，但优先使用数据库中的真实禅道配置
		s.logger.Warn("禅道未配置，使用Mock数据同步", zap.Error(err))
		result, syncErr := s.syncWithMockData(project)
		s.persistSyncDetails(syncLog.ID, result.Details)
		if syncErr != nil {
			s.finishSyncLog(syncLog.ID, model.IssueSyncStatusFailed, result, syncErr)
			return 0, syncErr
		}
		s.finishSyncLog(syncLog.ID, model.IssueSyncStatusSuccess, result, nil)
		return result.SyncedCount, nil
	}

	result, err := s.syncFromZentao(project, fullSync, baseURL, token)
	s.persistSyncDetails(syncLog.ID, result.Details)
	if err != nil {
		s.finishSyncLog(syncLog.ID, model.IssueSyncStatusFailed, result, err)
		return 0, err
	}

	s.finishSyncLog(syncLog.ID, model.IssueSyncStatusSuccess, result, nil)
	return result.SyncedCount, nil
}

// syncFromZentao 从禅道API同步（真实接入）
func (s *ZentaoService) syncFromZentao(project *model.Project, fullSync bool, baseURL, token string) (issueSyncResult, error) {
	base := strings.TrimRight(baseURL, "/")
	productID := *project.ZentaoProjectID

	product, err := s.getProductDetail(base, token, productID)
	if err != nil {
		return issueSyncResult{}, fmt.Errorf("获取禅道产品详情失败: %w", err)
	}

	branchFilterID, err := s.resolveBranchFilterID(project, product)
	if err != nil {
		return issueSyncResult{}, err
	}

	if !fullSync {
		// 增量：只获取最近24小时更新的
		since := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
		url := fmt.Sprintf("%s/api.php/v1/products/%d/bugs?limit=500&lastEditedDate>%s", base, productID, since)
		body, err := s.doZentaoGet(url, token)
		if err != nil {
			return issueSyncResult{}, fmt.Errorf("请求禅道API失败: %w", err)
		}
		return s.parseAndSyncBugs(body, project, product, branchFilterID, false, baseURL)
	}

	url := fmt.Sprintf("%s/api.php/v1/products/%d/bugs?limit=500", base, productID)
	body, err := s.doZentaoGet(url, token)
	if err != nil {
		return issueSyncResult{}, fmt.Errorf("请求禅道API失败: %w", err)
	}

	return s.parseAndSyncBugs(body, project, product, branchFilterID, true, baseURL)
}

func (s *ZentaoService) getZentaoConnection() (string, string, error) {
	baseURL := s.settingRepo.GetValue("zentao", "base_url")
	if baseURL == "" {
		return "", "", fmt.Errorf("未配置禅道 base_url")
	}

	token, err := s.settingSvc.GetZentaoToken()
	if err != nil {
		return "", "", fmt.Errorf("获取禅道Token失败: %w", err)
	}
	if token == "" {
		return "", "", fmt.Errorf("禅道Token为空")
	}

	return baseURL, token, nil
}

// doZentaoGet 发起禅道 GET 请求，自动尝试 /zentao/ 和无前缀两种路径
func (s *ZentaoService) doZentaoGet(urlWithPrefix, token string) ([]byte, error) {
	type attempt struct {
		url   string
		token string
	}

	// 先试原路径（带 /zentao/ 前缀），再试去掉 /zentao 的路径
	attempts := []attempt{{url: urlWithPrefix, token: token}}
	if idx := strings.Index(urlWithPrefix, "/zentao/"); idx >= 0 {
		fallback := urlWithPrefix[:idx] + urlWithPrefix[idx+len("/zentao"):]
		attempts = append(attempts, attempt{url: fallback, token: token})
	}

	var lastErr error
	refreshed := false
	currentToken := token

	for {
		unauthorized := false

		for _, a := range attempts {
			req, err := http.NewRequest("GET", a.url, nil)
			if err != nil {
				lastErr = err
				continue
			}
			req.Header.Set("Token", currentToken)

			resp, err := s.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized {
				unauthorized = true
				lastErr = fmt.Errorf("401: %s", a.url)
				continue
			}
			if resp.StatusCode == http.StatusNotFound {
				lastErr = fmt.Errorf("404: %s", a.url)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("禅道API返回错误 %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
				continue
			}
			return body, nil
		}

		if unauthorized && !refreshed {
			newToken, err := s.settingSvc.RefreshZentaoToken()
			if err != nil {
				return nil, fmt.Errorf("禅道认证失效且刷新Token失败: %w", err)
			}
			currentToken = newToken
			refreshed = true
			continue
		}

		return nil, lastErr
	}
}

type zentaoProductDetail struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func (s *ZentaoService) getProductDetail(baseURL, token string, productID int) (*zentaoProductDetail, error) {
	body, err := s.doZentaoGet(fmt.Sprintf("%s/api.php/v1/products/%d", strings.TrimRight(baseURL, "/"), productID), token)
	if err != nil {
		return nil, err
	}

	var product zentaoProductDetail
	if err := json.Unmarshal(body, &product); err != nil {
		return nil, fmt.Errorf("解析产品详情失败: %w", err)
	}
	return &product, nil
}

func (s *ZentaoService) resolveBranchFilterID(project *model.Project, product *zentaoProductDetail) (*int, error) {
	branchValue := strings.TrimSpace(project.ZentaoBranch)
	if branchValue == "" || strings.EqualFold(branchValue, "all") {
		return nil, nil
	}

	// 分支型产品本身已经是分支视角，直接用 products/{id}/bugs 的结果即可。
	if product != nil && product.Type == "branch" {
		return nil, nil
	}

	branchID, err := strconv.Atoi(branchValue)
	if err == nil {
		return &branchID, nil
	}

	return nil, fmt.Errorf("当前项目配置的禅道分支为“%s”，但 bugs 接口返回的是数值 branch ID；请先将项目分支改为分支 ID、主干(0)、所有(all)，或选择分支型产品", project.ZentaoBranch)
}

// parseAndSyncBugs 解析禅道 bug 列表并同步到数据库
func (s *ZentaoService) parseAndSyncBugs(body []byte, project *model.Project, product *zentaoProductDetail, branchFilterID *int, fullSync bool, baseURL string) (issueSyncResult, error) {
	var zentaoResp struct {
		Bugs []zentaoBug `json:"bugs"`
	}

	if err := json.Unmarshal(body, &zentaoResp); err != nil {
		return issueSyncResult{}, fmt.Errorf("解析禅道响应失败: %w", err)
	}

	zentaoIDs := make([]int, 0, len(zentaoResp.Bugs))
	incomingIssues := make([]model.Issue, 0, len(zentaoResp.Bugs))
	now := time.Now()

	for _, bug := range zentaoResp.Bugs {
		if branchFilterID != nil && bug.Branch != *branchFilterID {
			continue
		}
		zentaoIDs = append(zentaoIDs, bug.ID)

		reporter := ""
		reporterEmail := ""
		if bug.OpenedBy != nil {
			reporter = firstNonEmpty(bug.OpenedBy.Realname, bug.OpenedBy.Account)
			reporterEmail = bug.OpenedBy.Email
		}
		assignee := ""
		assigneeEmail := ""
		if bug.AssignedTo != nil {
			assignee = firstNonEmpty(bug.AssignedTo.Realname, bug.AssignedTo.Account)
			assigneeEmail = bug.AssignedTo.Email
		}
		issueBranch := strconv.Itoa(bug.Branch)
		if product != nil && product.Type == "branch" && project.ZentaoBranch != "" {
			issueBranch = project.ZentaoBranch
		}

		description := normalizeZentaoRichText(bug.Steps, baseURL)

		issue := &model.Issue{
			ZentaoID:     bug.ID,
			ProjectID:    project.ID,
			Title:        bug.Title,
			Description:  description,
			IssueType:    "bug",
			ZentaoStatus: mapZentaoStatus(bug.Status),
			Severity:     mapSeverity(bug.Severity),
			Priority:     int8(bug.Pri),
			Reporter:     reporter,
			ReporterEmail: reporterEmail,
			Assignee:     assignee,
			AssigneeEmail: assigneeEmail,
			Branch:       issueBranch,
			SyncedAt:     &now,
		}

		if bug.ResolvedDate != nil && *bug.ResolvedDate != "" {
			if t, err := time.Parse(time.RFC3339, *bug.ResolvedDate); err == nil {
				issue.ResolvedAt = &t
			}
		}
		if bug.LastEditedDate != nil && *bug.LastEditedDate != "" {
			if t, err := time.Parse(time.RFC3339, *bug.LastEditedDate); err == nil {
				issue.ZentaoUpdatedAt = &t
			}
		}

		incomingIssues = append(incomingIssues, *issue)
	}

	result, err := s.syncIssues(project.ID, incomingIssues, zentaoIDs, fullSync)
	if err != nil {
		return result, err
	}
	s.logger.Info("禅道同步完成",
		zap.Uint64("project_id", project.ID),
		zap.Int("synced", result.SyncedCount),
		zap.Int("added", result.AddedCount),
		zap.Int("updated", result.UpdatedCount),
		zap.Int("deleted", result.DeletedCount))
	return result, nil
}

func (s *ZentaoService) syncIssues(projectID uint64, incomingIssues []model.Issue, zentaoIDs []int, fullSync bool) (issueSyncResult, error) {
	existingIssues, err := s.issueRepo.FindByProjectAndZentaoIDs(projectID, zentaoIDs)
	if err != nil {
		return issueSyncResult{}, fmt.Errorf("查询已有问题单失败: %w", err)
	}

	existingMap := make(map[int]model.Issue, len(existingIssues))
	for _, issue := range existingIssues {
		existingMap[issue.ZentaoID] = issue
	}

	result := issueSyncResult{
		Details: make([]model.IssueSyncLogDetail, 0, len(incomingIssues)),
	}
	for i := range incomingIssues {
		issue := incomingIssues[i]
		existing, exists := existingMap[issue.ZentaoID]
		changes := []model.IssueSyncFieldChange{}
		if exists {
			changes = buildIssueFieldChanges(existing, issue)
		}

		if err := s.issueRepo.Upsert(&issue); err != nil {
			s.logger.Error("同步问题单失败", zap.Int("zentao_id", issue.ZentaoID), zap.Error(err))
			continue
		}

		if exists {
			if len(changes) > 0 {
				result.UpdatedCount++
				result.Details = append(result.Details, model.IssueSyncLogDetail{
					ProjectID:         projectID,
					IssueID:           &existing.ID,
					ZentaoID:          issue.ZentaoID,
					IssueTitle:        issue.Title,
					Action:            model.IssueSyncActionUpdated,
					ChangedFieldsJSON: model.EncodeIssueSyncFieldChanges(changes),
				})
			}
		} else {
			var issueID *uint64
			if issue.ID > 0 {
				issueID = &issue.ID
			}
			result.AddedCount++
			result.Details = append(result.Details, model.IssueSyncLogDetail{
				ProjectID:         projectID,
				IssueID:           issueID,
				ZentaoID:          issue.ZentaoID,
				IssueTitle:        issue.Title,
				Action:            model.IssueSyncActionAdded,
				ChangedFieldsJSON: model.EncodeIssueSyncFieldChanges(nil),
			})
		}

		result.SyncedCount++
	}

	if fullSync {
		deletedIssues, err := s.issueRepo.ListMissingByProject(projectID, zentaoIDs)
		if err != nil {
			return result, fmt.Errorf("查询失效问题单失败: %w", err)
		}

		deletedCount, err := s.issueRepo.DeleteMissingByProject(projectID, zentaoIDs)
		if err != nil {
			return result, fmt.Errorf("删除失效问题单失败: %w", err)
		}
		result.DeletedCount = int(deletedCount)

		for _, deletedIssue := range deletedIssues {
			issueID := deletedIssue.ID
			result.Details = append(result.Details, model.IssueSyncLogDetail{
				ProjectID:         projectID,
				IssueID:           &issueID,
				ZentaoID:          deletedIssue.ZentaoID,
				IssueTitle:        deletedIssue.Title,
				Action:            model.IssueSyncActionDeleted,
				ChangedFieldsJSON: model.EncodeIssueSyncFieldChanges(nil),
			})
		}
	}

	return result, nil
}

func buildIssueFieldChanges(existing model.Issue, incoming model.Issue) []model.IssueSyncFieldChange {
	changes := make([]model.IssueSyncFieldChange, 0, 8)
	appendIssueFieldChange(&changes, "title", "标题", existing.Title, incoming.Title)
	appendIssueFieldChange(&changes, "description", "描述", existing.Description, incoming.Description)
	appendIssueFieldChange(&changes, "zentao_status", "禅道状态", existing.ZentaoStatus, incoming.ZentaoStatus)
	appendIssueFieldChange(&changes, "severity", "严重程度", existing.Severity, incoming.Severity)
	appendIssueFieldChange(&changes, "priority", "优先级", strconv.Itoa(int(existing.Priority)), strconv.Itoa(int(incoming.Priority)))
	appendIssueFieldChange(&changes, "reporter", "提出人", existing.Reporter, incoming.Reporter)
	appendIssueFieldChange(&changes, "reporter_email", "提出人邮箱", existing.ReporterEmail, incoming.ReporterEmail)
	appendIssueFieldChange(&changes, "assignee", "负责人", existing.Assignee, incoming.Assignee)
	appendIssueFieldChange(&changes, "assignee_email", "负责人邮箱", existing.AssigneeEmail, incoming.AssigneeEmail)
	appendIssueFieldChange(&changes, "branch", "分支", existing.Branch, incoming.Branch)
	appendIssueFieldChange(&changes, "resolved_at", "解决时间", formatIssueSyncTime(existing.ResolvedAt), formatIssueSyncTime(incoming.ResolvedAt))
	return changes
}

func appendIssueFieldChange(changes *[]model.IssueSyncFieldChange, field, label, oldValue, newValue string) {
	if oldValue == newValue {
		return
	}

	*changes = append(*changes, model.IssueSyncFieldChange{
		Field:      field,
		FieldLabel: label,
		OldValue:   oldValue,
		NewValue:   newValue,
	})
}

func formatIssueSyncTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// syncWithMockData Mock同步（开发/测试用）
// [MOCK] 此方法为Mock实现，真实环境需替换为 syncFromZentao
func (s *ZentaoService) syncWithMockData(project *model.Project) (issueSyncResult, error) {
	now := time.Now()
	resolved := now.Add(-2 * time.Hour)

	mockBugs := []model.Issue{
		{
			ZentaoID:        1001,
			ProjectID:       project.ID,
			Title:           "[Mock] 用户登录后token未正确刷新",
			Description:     "用户登录后，在token过期前刷新页面，token未自动续期，导致用户被强制退出。",
			IssueType:       "bug",
			ZentaoStatus:    "resolved",
			TestStatus:      model.TestStatusPending,
			Severity:        "major",
			Priority:        2,
			Reporter:        "zhangsan",
			ReporterEmail:   "zhangsan@example.com",
			Assignee:        "lisi",
			AssigneeEmail:   "lisi@example.com",
			Branch:          "develop",
			ResolvedAt:      &resolved,
			ZentaoUpdatedAt: &now,
			SyncedAt:        &now,
		},
		{
			ZentaoID:        1002,
			ProjectID:       project.ID,
			Title:           "[Mock] 项目列表分页参数异常时返回500",
			Description:     "当page参数传入负数时，后端返回500错误，应返回参数校验错误。",
			IssueType:       "bug",
			ZentaoStatus:    "resolved",
			TestStatus:      model.TestStatusPending,
			Severity:        "normal",
			Priority:        3,
			Reporter:        "wangwu",
			ReporterEmail:   "wangwu@example.com",
			Assignee:        "lisi",
			AssigneeEmail:   "lisi@example.com",
			Branch:          "develop",
			ResolvedAt:      &resolved,
			ZentaoUpdatedAt: &now,
			SyncedAt:        &now,
		},
		{
			ZentaoID:     1003,
			ProjectID:    project.ID,
			Title:        "[Mock] 导出报告按钮在移动端不显示",
			Description:  "在手机浏览器访问时，导出报告按钮被遮挡，用户无法点击。",
			IssueType:    "bug",
			ZentaoStatus: "active",
			TestStatus:   model.TestStatusPending,
			Severity:     "minor",
			Priority:     4,
			Reporter:     "zhaoliu",
			Assignee:     "sunqi",
			Branch:       "develop",
			SyncedAt:     &now,
		},
	}

	zentaoIDs := make([]int, 0, len(mockBugs))
	for _, issue := range mockBugs {
		zentaoIDs = append(zentaoIDs, issue.ZentaoID)
	}

	result, err := s.syncIssues(project.ID, mockBugs, zentaoIDs, false)
	if err != nil {
		return result, err
	}

	s.logger.Info("Mock同步完成", zap.Uint64("project_id", project.ID), zap.Int("synced", result.SyncedCount))
	return result, nil
}

// SyncAllProjects 同步所有启用项目的问题单
func (s *ZentaoService) SyncAllProjects() {
	projects, err := s.projectRepo.GetAllActive()
	if err != nil {
		s.logger.Error("获取项目列表失败", zap.Error(err))
		return
	}

	for _, p := range projects {
		if p.ZentaoProjectID == nil {
			continue
		}
		count, err := s.SyncProjectIssues(p.ID, false)
		if err != nil {
			s.logger.Error("同步项目问题单失败", zap.Uint64("project_id", p.ID), zap.Error(err))
			continue
		}
		s.logger.Info("同步完成", zap.String("project", p.Name), zap.Int("count", count))
	}
}

func (s *ZentaoService) finishSyncLog(logID uint64, status string, result issueSyncResult, err error) {
	if logID == 0 {
		return
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if updateErr := s.syncLogRepo.UpdateResult(logID, status, result.AddedCount, result.UpdatedCount, result.DeletedCount, errMsg); updateErr != nil {
		s.logger.Error("更新同步日志失败", zap.Uint64("log_id", logID), zap.Error(updateErr))
	}
}

func (s *ZentaoService) persistSyncDetails(logID uint64, details []model.IssueSyncLogDetail) {
	if logID == 0 || len(details) == 0 {
		return
	}

	for i := range details {
		details[i].SyncLogID = logID
	}

	if err := s.syncLogRepo.BatchCreateDetails(details); err != nil {
		s.logger.Error("保存同步明细失败", zap.Uint64("log_id", logID), zap.Error(err))
	}
}

func mapZentaoStatus(status string) string {
	switch status {
	case "active":
		return "active"
	case "resolved":
		return "resolved"
	case "closed":
		return "closed"
	default:
		return status
	}
}

func mapSeverity(level int) string {
	switch level {
	case 1:
		return "critical"
	case 2:
		return "major"
	case 3:
		return "normal"
	default:
		return "minor"
	}
}

var zentaoRelativeLinkRegex = regexp.MustCompile(`(?i)(src|href)="(/[^"#?][^"]*)"`)

func normalizeZentaoRichText(raw, baseURL string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		return raw
	}

	// 将禅道富文本中的相对链接转为绝对链接，避免前端详情页图片加载失败。
	return zentaoRelativeLinkRegex.ReplaceAllString(trimmed, `$1="`+base+`$2"`)
}
