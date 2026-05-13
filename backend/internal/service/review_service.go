package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const gitApproveTimeout = 20 * time.Second

type ReviewService struct {
	reviewRepo     *repository.ReviewRepo
	testTaskRepo   *repository.TestTaskRepo
	issueRepo      *repository.IssueRepo
	gitOpsService  *GitOpsService
	notifyService  *NotificationService
	zentaoProxy    *ZentaoProxyService
	genTestService *GenTestService
	logger         *zap.Logger
}

func NewReviewService(logger *zap.Logger) *ReviewService {
	return &ReviewService{
		reviewRepo:     repository.NewReviewRepo(),
		testTaskRepo:   repository.NewTestTaskRepo(),
		issueRepo:      repository.NewIssueRepo(),
		gitOpsService:  NewGitOpsService(logger),
		notifyService:  NewNotificationService(logger),
		zentaoProxy:    NewZentaoProxyService(logger),
		genTestService: NewGenTestService(logger),
		logger:         logger,
	}
}

// List Review任务列表
func (s *ReviewService) List(req *dto.ReviewListQuery) ([]model.ReviewTask, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize
	return s.reviewRepo.List(req.ProjectID, req.TaskID, req.Status, req.ReviewerID, offset, req.PageSize)
}

// GetDetail 获取Review详情(包含测试用例/脚本/文档/审核记录)
func (s *ReviewService) GetDetail(reviewID uint64) (*dto.ReviewDetailResponse, error) {
	rt, err := s.reviewRepo.GetByID(reviewID)
	if err != nil {
		return nil, errors.New("Review任务不存在")
	}

	// 获取关联的测试用例
	cases, _ := s.testTaskRepo.GetTestCasesByTaskID(rt.TestTaskID)
	scripts, _ := s.testTaskRepo.GetTestScriptsByTaskID(rt.TestTaskID)
	docs, _ := s.testTaskRepo.GetTestDocsByTaskID(rt.TestTaskID)
	records, _ := s.reviewRepo.GetRecordsByTaskID(reviewID)

	// 组装响应
	resp := &dto.ReviewDetailResponse{
		ID:         rt.ID,
		Title:      rt.Title,
		Status:     rt.Status,
		IssueTitle: "",
	}

	if rt.Issue != nil {
		resp.IssueTitle = rt.Issue.Title
	}

	resp.TestCases = make([]dto.TestCaseVO, 0, len(cases))
	for _, tc := range cases {
		resp.TestCases = append(resp.TestCases, dto.TestCaseVO{
			ID:             tc.ID,
			Title:          tc.Title,
			Category:       tc.Category,
			Precondition:   tc.Precondition,
			Steps:          tc.Steps,
			Expected:       tc.Expected,
			SelfTestResult: tc.SelfTestResult,
			Source:         tc.Source,
		})
	}

	resp.TestScripts = make([]dto.TestScriptVO, 0, len(scripts))
	for _, ts := range NormalizeTestScripts(scripts) {
		resp.TestScripts = append(resp.TestScripts, dto.TestScriptVO{
			ID:          ts.ID,
			FilePath:    ts.FilePath,
			FileContent: ts.FileContent,
			Language:    ts.Language,
			Source:      ts.Source,
		})
	}

	resp.TestDocs = make([]dto.TestDocVO, 0, len(docs))
	for _, td := range docs {
		resp.TestDocs = append(resp.TestDocs, dto.TestDocVO{
			ID:      td.ID,
			Title:   td.Title,
			Content: td.Content,
			DocType: td.DocType,
			Source:  td.Source,
		})
	}

	resp.Records = make([]dto.ReviewRecordVO, 0, len(records))
	for _, r := range records {
		name := ""
		if r.Reviewer != nil {
			name = r.Reviewer.RealName
		}
		resp.Records = append(resp.Records, dto.ReviewRecordVO{
			ID:           r.ID,
			ReviewerName: name,
			Action:       r.Action,
			Comment:      r.Comment,
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return resp, nil
}

// DoReview 执行审核操作
func (s *ReviewService) DoReview(reviewID, reviewerID uint64, req *dto.ReviewActionRequest) (*dto.ReviewActionResult, error) {
	rt, err := s.reviewRepo.GetByID(reviewID)
	if err != nil {
		return nil, errors.New("Review任务不存在")
	}

	if rt.Status != model.ReviewStatusPending && rt.Status != model.ReviewStatusChangesRequested {
		return nil, errors.New("Review任务当前状态不允许审核")
	}

	if (req.Action == "fail_regression" || req.Action == "request_changes") && strings.TrimSpace(req.Comment) == "" {
		return nil, errors.New("审核意见不能为空")
	}

	result := &dto.ReviewActionResult{}

	// 创建审核记录
	now := time.Now()
	rt.ReviewerID = &reviewerID
	rt.ReviewedAt = &now
	rt.ReviewNote = req.Comment

	gitSummary := "未执行"
	closeZentaoBug := false
	activateZentaoBug := false
	devFlowFailureType := ""

	switch req.Action {
	case "approve":
		if err := s.pushReviewedContentWithTimeout(rt); err != nil {
			return nil, fmt.Errorf("Git推送失败，审核未完成: %w", err)
		}
		gitSummary = "已推送到Git仓库"
		rt.Status = model.ReviewStatusApproved
		// 更新问题单测试状态为"审核通过"
		_ = s.issueRepo.ForceUpdateTestStatus(rt.IssueID, model.TestStatusReviewApproved)
		// 记录状态变更日志
		s.logStatusChange(rt.IssueID, model.TestStatusReviewPending, model.TestStatusReviewApproved, "manual", &reviewerID, "Review审核通过")
		closeZentaoBug = true

	case "reject":
		rt.Status = model.ReviewStatusRejected
		gitSummary = "审核驳回，未推送"
		_ = s.issueRepo.ForceUpdateTestStatus(rt.IssueID, model.TestStatusReviewRejected)
		s.logStatusChange(rt.IssueID, model.TestStatusReviewPending, model.TestStatusReviewRejected, "manual", &reviewerID, fmt.Sprintf("Review驳回: %s", req.Comment))
		devFlowFailureType = "review_rejected"

	case "request_changes":
		rt.Status = model.ReviewStatusChangesRequested
		gitSummary = "要求修改，未推送"
		_ = s.issueRepo.ForceUpdateTestStatus(rt.IssueID, model.TestStatusReviewRejected)
		s.logStatusChange(rt.IssueID, model.TestStatusReviewPending, model.TestStatusReviewRejected, "manual", &reviewerID, fmt.Sprintf("Review驳回(需修改): %s", req.Comment))
		// 驳回后创建新的运行任务并异步重新生成测试，不复用旧 task。
		newTask, createErr := s.createRegeneratedTask(rt, reviewerID)
		if createErr != nil {
			return nil, fmt.Errorf("创建重新生成任务失败: %w", createErr)
		}
		newTaskID := newTask.ID
		result.NewTaskID = &newTaskID
		go s.regenerateTestAsync(newTaskID)

	case "fail_regression":
		gitSummary = "回归失败，未推送"
		rt.Status = model.ReviewStatusRejected
		_ = s.issueRepo.ForceUpdateTestStatus(rt.IssueID, model.TestStatusReviewRejected)
		s.logStatusChange(rt.IssueID, model.TestStatusReviewPending, model.TestStatusReviewRejected, "manual", &reviewerID, "回归测试失败，问题单重新激活")
		activateZentaoBug = true
		devFlowFailureType = "regression_failed"

	case "comment":
		// 仅评论，不改变状态
		gitSummary = "仅评论，未推送"

	case "reject_and_mark_error":
		rt.Status = model.ReviewStatusRejected
		gitSummary = "审核驳回并标记异常，未推送"
		_ = s.issueRepo.ForceUpdateTestStatus(rt.IssueID, model.TestStatusError)
		s.logStatusChange(rt.IssueID, model.TestStatusReviewPending, model.TestStatusError, "manual", &reviewerID, fmt.Sprintf("审核驳回并标记为异常: %s", req.Comment))
	}

	record := &model.ReviewRecord{
		ReviewTaskID: reviewID,
		ReviewerID:   reviewerID,
		Action:       req.Action,
		Comment:      req.Comment,
	}
	if err := s.reviewRepo.CreateRecord(record); err != nil {
		return nil, err
	}

	if err := s.reviewRepo.Update(rt); err != nil {
		return nil, err
	}

	if closeZentaoBug {
		go s.closeZentaoBugAsync(rt.IssueID, rt.ID, req.Comment)
	}
	if activateZentaoBug {
		go s.activateZentaoBugAsync(rt.IssueID, rt.ID, req.Comment)
	}
	if devFlowFailureType != "" {
		go s.notifyDevFlowFailureAsync(rt.IssueID, devFlowFailureType, req.Comment)
	}

	if req.Action == "approve" || req.Action == "reject" || req.Action == "fail_regression" || req.Action == "reject_and_mark_error" {
		s.notifyService.SendReviewResult(rt, req.Action, gitSummary)
	}

	return result, nil
}

func (s *ReviewService) createRegeneratedTask(rt *model.ReviewTask, reviewerID uint64) (*model.TestTask, error) {
	if rt == nil {
		return nil, errors.New("Review任务不存在")
	}

	currentTask, err := s.testTaskRepo.GetByID(rt.TestTaskID)
	if err != nil {
		return nil, fmt.Errorf("原测试任务不存在: %w", err)
	}

	createdBy := reviewerID
	workflowName := strings.TrimSpace(currentTask.SkillName)
	return s.genTestService.CreatePendingTask(rt.IssueID, currentTask.AgentID, &createdBy, workflowName)
}

func (s *ReviewService) regenerateTestAsync(taskID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := s.genTestService.RunTask(ctx, taskID); err != nil {
		s.logger.Error("驳回后重新生成测试失败", zap.Uint64("task_id", taskID), zap.Error(err))
	}
}

func (s *ReviewService) closeZentaoBugAsync(issueID, reviewID uint64, comment string) {
	issue, err := s.issueRepo.GetByID(issueID)
	if err != nil || issue.ZentaoID <= 0 {
		return
	}
	closeComment := fmt.Sprintf("[AutoTestFlow] 回归测试确认成功，Review#%d 审核通过。%s", reviewID, comment)
	if err := s.zentaoProxy.CloseBug(issue.ZentaoID, closeComment); err != nil {
		s.logger.Warn("调用禅道关闭Bug API失败", zap.Error(err), zap.Uint64("issueID", issueID))
	}
}

func (s *ReviewService) activateZentaoBugAsync(issueID, reviewID uint64, comment string) {
	issue, err := s.issueRepo.GetByID(issueID)
	if err != nil || issue.ZentaoID <= 0 {
		return
	}
	activateComment := fmt.Sprintf("[AutoTestFlow] 回归测试失败，Review#%d 确认失败。%s", reviewID, comment)
	if err := s.zentaoProxy.ActivateBug(issue.ZentaoID, activateComment); err != nil {
		s.logger.Warn("调用禅道激活Bug API失败", zap.Error(err), zap.Uint64("issueID", issueID))
	}
}

func (s *ReviewService) notifyDevFlowFailureAsync(issueID uint64, failureType, comment string) {
	issue, err := s.issueRepo.GetByID(issueID)
	if err != nil || issue.DevTaskID == "" {
		return
	}
	if err := s.notifyService.NotifyDevFlowFailure(issue.DevTaskID, issue, failureType, comment); err != nil {
		s.logger.Warn("通知研发流水线失败", zap.Error(err), zap.Uint64("issueID", issueID))
	}
}

func (s *ReviewService) logStatusChange(issueID uint64, oldStatus, newStatus, triggerType string, operatorID *uint64, remark string) {
	log := &model.IssueStatusLog{
		IssueID:     issueID,
		Field:       "test_status",
		OldValue:    oldStatus,
		NewValue:    newStatus,
		TriggerType: triggerType,
		OperatorID:  operatorID,
		Remark:      remark,
	}
	_ = s.issueRepo.CreateStatusLog(log)
}

func (s *ReviewService) pushReviewedContentWithTimeout(reviewTask *model.ReviewTask) error {
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- s.gitOpsService.PushReviewedContent(reviewTask)
	}()

	select {
	case err := <-resultCh:
		return err
	case <-time.After(gitApproveTimeout):
		return fmt.Errorf("Git操作超时(%s)，请检查仓库连接或本地工作区状态", gitApproveTimeout)
	}
}
