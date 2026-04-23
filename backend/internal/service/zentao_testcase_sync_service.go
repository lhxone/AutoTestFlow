package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

type ZentaoTestCaseSyncService struct {
	tcRepo      *repository.ZentaoTestCaseRepo
	projectRepo *repository.ProjectRepo
	settingRepo *repository.SettingRepo
	settingSvc  *SettingService
	zentaoSvc   *ZentaoService
	syncLogRepo *repository.IssueSyncLogRepo
	logger      *zap.Logger
}

func NewZentaoTestCaseSyncService(logger *zap.Logger) *ZentaoTestCaseSyncService {
	return &ZentaoTestCaseSyncService{
		tcRepo:      repository.NewZentaoTestCaseRepo(),
		projectRepo: repository.NewProjectRepo(),
		settingRepo: repository.NewSettingRepo(),
		settingSvc:  NewSettingService(logger),
		zentaoSvc:   NewZentaoService(logger),
		syncLogRepo: repository.NewIssueSyncLogRepo(),
		logger:      logger,
	}
}

type zentaoTestCase struct {
	ID           int            `json:"id"`
	Product      int            `json:"product"`
	Branch       int            `json:"branch"`
	Module       int            `json:"module"`
	Story        int            `json:"story"`
	StoryVersion int            `json:"storyVersion"`
	Title        string         `json:"title"`
	Precondition string         `json:"precondition"`
	Keywords     string         `json:"keywords"`
	Pri          int            `json:"pri"`
	Type         string         `json:"type"`
	Stage        string         `json:"stage"`
	Status       string         `json:"status"`
	OpenedBy     *zentaoUserRef `json:"openedBy"`
	OpenedDate   *string        `json:"openedDate"`
	FromBug      int            `json:"fromBug"`
	FromCaseID   int            `json:"fromCaseID"`
	Steps        []zentaoStep   `json:"steps"`
}

type zentaoStep struct {
	ID     int    `json:"id"`
	Parent int    `json:"parent"`
	Case   int    `json:"case"`
	Type   string `json:"type"`
	Desc   string `json:"desc"`
	Expect string `json:"expect"`
}

type testCaseSyncResult struct {
	SyncedCount  int
	AddedCount   int
	UpdatedCount int
	DeletedCount int
	Details      []model.IssueSyncLogDetail
}

// StartAsyncSync 异步触发用例同步，立即返回 syncLogID
func (s *ZentaoTestCaseSyncService) StartAsyncSync(projectID uint64, fullSync bool) uint64 {
	syncLog := &model.IssueSyncLog{
		ProjectID: projectID,
		SyncType:  model.SyncTypeTestCase,
		Status:    model.IssueSyncStatusRunning,
		FullSync:  fullSync,
	}
	_ = s.syncLogRepo.Create(syncLog)

	go s.executeSync(projectID, fullSync, syncLog.ID)

	return syncLog.ID
}

// executeSync 执行同步逻辑（内部方法）
func (s *ZentaoTestCaseSyncService) executeSync(projectID uint64, fullSync bool, syncLogID uint64) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, testCaseSyncResult{}, fmt.Errorf("项目不存在: %w", err))
		return
	}

	if project.ZentaoProjectID == nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, testCaseSyncResult{}, fmt.Errorf("项目未配置禅道项目ID"))
		return
	}

	baseURL, token, err := s.getZentaoConnection()
	if err != nil {
		s.logger.Warn("禅道未配置，使用Mock数据", zap.Error(err))
		result, syncErr := s.syncTestCasesWithMockData(project)
		s.persistSyncDetails(syncLogID, result.Details)
		if syncErr != nil {
			s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, result, syncErr)
			return
		}
		s.finishSyncLog(syncLogID, model.IssueSyncStatusSuccess, result, nil)
		return
	}

	result, err := s.syncTestCasesFromZentao(project, fullSync, baseURL, token)
	s.persistSyncDetails(syncLogID, result.Details)
	if err != nil {
		s.finishSyncLog(syncLogID, model.IssueSyncStatusFailed, result, err)
		return
	}

	s.finishSyncLog(syncLogID, model.IssueSyncStatusSuccess, result, nil)
}

// SyncTestCases 同步指定产品的用例（同步版本）
func (s *ZentaoTestCaseSyncService) SyncTestCases(projectID uint64, fullSync bool) (testCaseSyncResult, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return testCaseSyncResult{}, fmt.Errorf("项目不存在: %w", err)
	}

	if project.ZentaoProjectID == nil {
		return testCaseSyncResult{}, fmt.Errorf("项目未配置禅道项目ID")
	}

	baseURL, token, err := s.getZentaoConnection()
	if err != nil {
		s.logger.Warn("禅道未配置，使用Mock数据", zap.Error(err))
		return s.syncTestCasesWithMockData(project)
	}

	return s.syncTestCasesFromZentao(project, fullSync, baseURL, token)
}

func (s *ZentaoTestCaseSyncService) getZentaoConnection() (string, string, error) {
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

func (s *ZentaoTestCaseSyncService) syncTestCasesFromZentao(project *model.Project, fullSync bool, baseURL, token string) (testCaseSyncResult, error) {
	base := strings.TrimRight(baseURL, "/")
	productID := *project.ZentaoProjectID

	url := fmt.Sprintf("%s/api.php/v1/products/%d/testcases?limit=500", base, productID)
	body, err := s.zentaoSvc.DoZentaoGet(url, token)
	if err != nil {
		return testCaseSyncResult{}, fmt.Errorf("请求禅道用例API失败: %w", err)
	}

	s.logger.Info("禅道用例API响应", zap.Int("product_id", productID), zap.Int("body_len", len(body)), zap.ByteString("raw_body", body[:min(len(body), 500)]))

	return s.parseAndSyncTestCases(body, project, fullSync, baseURL)
}

func (s *ZentaoTestCaseSyncService) parseAndSyncTestCases(body []byte, project *model.Project, fullSync bool, baseURL string) (testCaseSyncResult, error) {
	var zentaoResp struct {
		TestCases []zentaoTestCase `json:"testcases"`
	}

	if err := json.Unmarshal(body, &zentaoResp); err != nil {
		return testCaseSyncResult{}, fmt.Errorf("解析禅道用例响应失败: %w", err)
	}

	zentaoIDs := make([]int, 0, len(zentaoResp.TestCases))
	incomingCases := make([]model.ZentaoTestCase, 0, len(zentaoResp.TestCases))
	now := time.Now()

	for _, tc := range zentaoResp.TestCases {
		zentaoIDs = append(zentaoIDs, tc.ID)

		openedBy := ""
		if tc.OpenedBy != nil {
			openedBy = firstNonEmpty(tc.OpenedBy.Realname, tc.OpenedBy.Account)
		}

		stepsStr := ""
		expectedStr := ""
		if len(tc.Steps) > 0 {
			for _, step := range tc.Steps {
				if step.Desc != "" {
					stepsStr += step.Desc + "\n"
				}
				if step.Expect != "" {
					expectedStr += step.Expect + "\n"
				}
			}
			stepsStr = strings.TrimSpace(stepsStr)
			expectedStr = strings.TrimSpace(expectedStr)
		}

		testCase := &model.ZentaoTestCase{
			ZentaoID:     tc.ID,
			ProductID:    uint64(tc.Product),
			Title:        tc.Title,
			Precondition: tc.Precondition,
			Keywords:     tc.Keywords,
			Priority:     int8(tc.Pri),
			Type:         tc.Type,
			Stage:        tc.Stage,
			Status:       tc.Status,
			Branch:       strconv.Itoa(tc.Branch),
			Module:       strconv.Itoa(tc.Module),
			Steps:        stepsStr,
			Expected:     expectedStr,
			OpenedBy:     openedBy,
			TestStatus:   model.ZentaoTestCaseStatusSynced,
			SyncedAt:     &now,
		}

		if tc.OpenedDate != nil && *tc.OpenedDate != "" {
			if t, err := time.Parse(time.RFC3339, *tc.OpenedDate); err == nil {
				testCase.ZentaoUpdatedAt = &t
			}
		}

		incomingCases = append(incomingCases, *testCase)
	}

	result, err := s.syncTestCases(project.ID, incomingCases, zentaoIDs, fullSync)
	if err != nil {
		return result, err
	}

	s.logger.Info("禅道用例同步完成",
		zap.Uint64("project_id", project.ID),
		zap.Int("synced", result.SyncedCount),
		zap.Int("added", result.AddedCount),
		zap.Int("updated", result.UpdatedCount),
		zap.Int("deleted", result.DeletedCount))

	return result, nil
}

func (s *ZentaoTestCaseSyncService) syncTestCases(productID uint64, incomingCases []model.ZentaoTestCase, zentaoIDs []int, fullSync bool) (testCaseSyncResult, error) {
	existingCases, err := s.tcRepo.FindByProductAndZentaoIDs(productID, zentaoIDs)
	if err != nil {
		return testCaseSyncResult{}, fmt.Errorf("查询已有用例失败: %w", err)
	}

	existingMap := make(map[int]model.ZentaoTestCase, len(existingCases))
	for _, tc := range existingCases {
		existingMap[tc.ZentaoID] = tc
	}

	result := testCaseSyncResult{}
	details := make([]model.IssueSyncLogDetail, 0)

	for i := range incomingCases {
		tc := incomingCases[i]
		_, exists := existingMap[tc.ZentaoID]

		if err := s.tcRepo.Upsert(&tc); err != nil {
			s.logger.Error("同步用例失败", zap.Int("zentao_id", tc.ZentaoID), zap.Error(err))
			continue
		}

		if exists {
			result.UpdatedCount++
			// 记录变更详情
			details = append(details, model.IssueSyncLogDetail{
				ProjectID:  productID,
				SyncType:   model.SyncTypeTestCase,
				ZentaoID:   tc.ZentaoID,
				IssueTitle: tc.Title,
				Action:     model.IssueSyncActionUpdated,
			})
		} else {
			result.AddedCount++
			details = append(details, model.IssueSyncLogDetail{
				ProjectID:  productID,
				SyncType:   model.SyncTypeTestCase,
				ZentaoID:   tc.ZentaoID,
				IssueTitle: tc.Title,
				Action:     model.IssueSyncActionAdded,
			})
		}
		result.SyncedCount++
	}

	if fullSync {
		deletedIDs, err := s.tcRepo.FindMissingZentaoIDsByProduct(productID, zentaoIDs)
		if err != nil {
			s.logger.Error("查找失效用例失败", zap.Error(err))
		} else {
			for _, zentaoID := range deletedIDs {
				// 获取被删除用例的标题
				if tc, ok := existingMap[zentaoID]; ok {
					details = append(details, model.IssueSyncLogDetail{
						ProjectID:  productID,
						SyncType:   model.SyncTypeTestCase,
						ZentaoID:   zentaoID,
						IssueTitle: tc.Title,
						Action:     model.IssueSyncActionDeleted,
					})
				}
			}

			deletedCount, err := s.tcRepo.DeleteMissingByProduct(productID, zentaoIDs)
			if err != nil {
				return result, fmt.Errorf("删除失效用例失败: %w", err)
			}
			result.DeletedCount = int(deletedCount)
		}
	}

	result.Details = details
	return result, nil
}

// syncTestCasesWithMockData Mock同步
func (s *ZentaoTestCaseSyncService) syncTestCasesWithMockData(project *model.Project) (testCaseSyncResult, error) {
	now := time.Now()

	mockCases := []model.ZentaoTestCase{
		{
			ZentaoID:     2001,
			ProductID:    project.ID,
			Title:        "[Mock] 用户登录功能测试",
			Precondition: "用户已注册且账号处于正常状态",
			Priority:     1,
			Type:         "feature",
			Stage:        "system",
			Status:       "normal",
			TestStatus:   model.ZentaoTestCaseStatusSynced,
			Branch:       "develop",
			Steps:        "1. 打开登录页面\n2. 输入正确的用户名和密码\n3. 点击登录按钮",
			Expected:     "登录成功，跳转到首页",
			OpenedBy:     "zhangsan",
			SyncedAt:     &now,
		},
		{
			ZentaoID:     2002,
			ProductID:    project.ID,
			Title:        "[Mock] 项目创建功能测试",
			Precondition: "用户已登录且具有项目管理权限",
			Priority:     2,
			Type:         "feature",
			Stage:        "system",
			Status:       "normal",
			TestStatus:   model.ZentaoTestCaseStatusSynced,
			Branch:       "develop",
			Steps:        "1. 进入项目管理页面\n2. 点击创建项目按钮\n3. 填写项目信息\n4. 点击保存",
			Expected:     "项目创建成功，在列表中显示",
			OpenedBy:     "lisi",
			SyncedAt:     &now,
		},
		{
			ZentaoID:     2003,
			ProductID:    project.ID,
			Title:        "[Mock] 密码强度校验测试",
			Precondition: "用户处于注册或修改密码页面",
			Priority:     3,
			Type:         "security",
			Stage:        "system",
			Status:       "normal",
			TestStatus:   model.ZentaoTestCaseStatusSynced,
			Branch:       "develop",
			Steps:        "1. 输入弱密码（如123）\n2. 提交表单",
			Expected:     "提示密码强度不足，拒绝提交",
			OpenedBy:     "wangwu",
			SyncedAt:     &now,
		},
	}

	zentaoIDs := make([]int, 0, len(mockCases))
	for _, tc := range mockCases {
		zentaoIDs = append(zentaoIDs, tc.ZentaoID)
	}

	result, err := s.syncTestCases(project.ID, mockCases, zentaoIDs, false)
	if err != nil {
		return result, err
	}

	s.logger.Info("Mock用例同步完成", zap.Uint64("project_id", project.ID), zap.Int("synced", result.SyncedCount))
	return result, nil
}

// finishSyncLog 更新同步日志状态
func (s *ZentaoTestCaseSyncService) finishSyncLog(logID uint64, status string, result testCaseSyncResult, err error) {
	if logID == 0 {
		return
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if updateErr := s.syncLogRepo.UpdateResult(logID, status, result.AddedCount, result.UpdatedCount, result.DeletedCount, errMsg); updateErr != nil {
		s.logger.Error("更新用例同步日志失败", zap.Uint64("log_id", logID), zap.Error(updateErr))
	}
}

// persistSyncDetails 保存同步明细
func (s *ZentaoTestCaseSyncService) persistSyncDetails(logID uint64, details []model.IssueSyncLogDetail) {
	if logID == 0 || len(details) == 0 {
		return
	}

	for i := range details {
		details[i].SyncLogID = logID
	}

	if err := s.syncLogRepo.BatchCreateDetails(details); err != nil {
		s.logger.Error("保存用例同步明细失败", zap.Uint64("log_id", logID), zap.Error(err))
	}
}
