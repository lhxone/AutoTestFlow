package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

type NotificationService struct {
	executionRepo *repository.ExecutionRepo
	userRepo      *repository.UserRepo
	projectRepo   *repository.ProjectRepo
	testTaskRepo  *repository.TestTaskRepo
	settingRepo   *repository.SettingRepo
	logger        *zap.Logger
}

func NewNotificationService(logger *zap.Logger) *NotificationService {
	return &NotificationService{
		executionRepo: repository.NewExecutionRepo(),
		userRepo:      repository.NewUserRepo(),
		projectRepo:   repository.NewProjectRepo(),
		testTaskRepo:  repository.NewTestTaskRepo(),
		settingRepo:   repository.NewSettingRepo(),
		logger:        logger,
	}
}

// SendTestReport 发送测试报告通知
func (s *NotificationService) SendTestReport(report *model.TestReport, recipients []string) {
	cfg := LoadMailConfig()
	recipients = uniqueEmails(append(recipients, cfg.DefaultRecipients...))
	subjectTemplate := s.loadTemplate("test_report_subject_template")
	bodyTemplate := s.loadTemplate("test_report_body_template")
	values := map[string]string{
		"title":            report.Title,
		"summary":          report.Summary,
		"total_cases":      fmt.Sprintf("%d", report.TotalCases),
		"passed_cases":     fmt.Sprintf("%d", report.PassedCases),
		"failed_cases":     fmt.Sprintf("%d", report.FailedCases),
		"pass_rate":        formatPassRate(report.PassRate),
		"has_intervention": map[bool]string{true: "是", false: "否"}[report.HasIntervention == 1],
		"report_url":       report.ReportURL,
	}
	subject := renderMailTemplate(subjectTemplate, values)
	body := renderMailTemplate(bodyTemplate, values)
	if cfg.Host == "" {
		s.logger.Warn("邮件服务未配置，跳过发送", zap.Strings("recipients", recipients))
		for _, recipient := range recipients {
			s.logEmail(nil, &report.ID, recipient, subject, body, "skipped", "")
		}
		return
	}

	for _, recipient := range recipients {
		go s.sendMail(cfg, nil, &report.ID, recipient, subject, body)
	}
}

// SendReviewResult 发送 Review 结果通知
func (s *NotificationService) SendReviewResult(reviewTask *model.ReviewTask, action, gitSummary string) {
	recipients := s.resolveReviewRecipients(reviewTask)
	cfg := LoadMailConfig()
	recipients = uniqueEmails(append(recipients, cfg.DefaultRecipients...))
	projectName := ""
	if project, err := s.projectRepo.GetByID(reviewTask.ProjectID); err == nil {
		projectName = project.Name
	}
	values := map[string]string{
		"title":        reviewTask.Title,
		"status":       action,
		"review_note":  escapeEmpty(reviewTask.ReviewNote),
		"git_summary":  escapeEmpty(gitSummary),
		"issue_title":  reviewTask.Title,
		"project_name": escapeEmpty(projectName),
		"review_id":    fmt.Sprintf("%d", reviewTask.ID),
	}

	subject := renderMailTemplate(s.loadTemplate("review_result_subject_template"), values)
	body := renderMailTemplate(s.loadTemplate("review_result_body_template"), values)

	if cfg.Host == "" {
		s.logger.Warn("邮件服务未配置，跳过 Review 通知", zap.Strings("recipients", recipients))
		for _, recipient := range recipients {
			s.logEmail(&reviewTask.ID, nil, recipient, subject, body, "skipped", "")
		}
		return
	}

	for _, recipient := range recipients {
		go s.sendMail(cfg, &reviewTask.ID, nil, recipient, subject, body)
	}
}

func (s *NotificationService) resolveReviewRecipients(reviewTask *model.ReviewTask) []string {
	recipients := make([]string, 0, 4)

	if reviewTask.ReviewerID != nil {
		if user, err := s.userRepo.GetByID(*reviewTask.ReviewerID); err == nil {
			recipients = append(recipients, user.Email)
		}
	}

	if reviewTask.SubmittedBy != nil {
		if user, err := s.userRepo.GetByID(*reviewTask.SubmittedBy); err == nil {
			recipients = append(recipients, user.Email)
		}
	}

	if project, err := s.projectRepo.GetByID(reviewTask.ProjectID); err == nil && project.Owner != nil {
		recipients = append(recipients, project.Owner.Email)
	}

	if task, err := s.testTaskRepo.GetByID(reviewTask.TestTaskID); err == nil && task.CreatedBy != nil {
		if user, userErr := s.userRepo.GetByID(*task.CreatedBy); userErr == nil {
			recipients = append(recipients, user.Email)
		}
	}

	return uniqueEmails(recipients)
}

func (s *NotificationService) loadTemplate(key string) string {
	value := strings.TrimSpace(s.settingRepo.GetValue("mail", key))
	if value != "" {
		return value
	}
	return defaultMailTemplate(key)
}

func (s *NotificationService) sendMail(cfg MailConfig, reviewID, reportID *uint64, recipient, subject, body string) {
	m := gomail.NewMessage()
	m.SetHeader("From", cfg.From)
	m.SetHeader("To", recipient)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if cfg.UseSSL {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		s.logger.Error("发送邮件失败", zap.String("to", recipient), zap.Error(err))
		s.logEmail(reviewID, reportID, recipient, subject, body, "failed", err.Error())
		return
	}

	s.logEmail(reviewID, reportID, recipient, subject, body, "sent", "")
}

func (s *NotificationService) logEmail(reviewID, reportID *uint64, recipient, subject, body, status, errMsg string) {
	now := time.Now()
	logEntry := &model.NotificationLog{
		ReportID:     reportID,
		Channel:      "email",
		Recipient:    recipient,
		Subject:      subject,
		Content:      body,
		Status:       status,
		ErrorMessage: errMsg,
	}
	if status == "sent" {
		logEntry.SentAt = &now
	}
	_ = reviewID
	_ = s.executionRepo.CreateNotificationLog(logEntry)
}

func uniqueEmails(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, trimmed)
	}
	return result
}

func escapeEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// NotifyDevFlowFailure 通知研发流水线测试失败
func (s *NotificationService) NotifyDevFlowFailure(devTaskID string, issue *model.Issue, failureType, comment string) error {
	if devTaskID == "" {
		s.logger.Debug("dev_task_id为空，跳过通知研发流水线")
		return nil
	}

	callbackURL := s.settingRepo.GetValue("integration", "devflow_callback_url")
	if callbackURL == "" {
		s.logger.Warn("研发流水线回调URL未配置，跳过通知")
		return nil
	}

	payload := map[string]interface{}{
		"dev_task_id":   devTaskID,
		"zentao_id":     issue.ZentaoID,
		"issue_title":   issue.Title,
		"failure_type":  failureType,
		"comment":       comment,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", callbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("通知研发流水线失败", zap.Error(err), zap.String("dev_task_id", devTaskID))
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.logger.Warn("研发流水线回调返回错误状态", zap.Int("status", resp.StatusCode), zap.String("dev_task_id", devTaskID))
		return fmt.Errorf("回调返回状态码: %d", resp.StatusCode)
	}

	s.logger.Info("已通知研发流水线测试失败",
		zap.String("dev_task_id", devTaskID),
		zap.Int("zentao_id", issue.ZentaoID),
		zap.String("failure_type", failureType))

	return nil
}
