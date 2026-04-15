package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

type SettingService struct {
	settingRepo *repository.SettingRepo
	logger      *zap.Logger
	httpClient  *http.Client
}

func NewSettingService(logger *zap.Logger) *SettingService {
	return &SettingService{
		settingRepo: repository.NewSettingRepo(),
		logger:      logger,
		// 跳过TLS验证(禅道/GitLab内网可能用自签证书)
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// GetSettings 获取某分类的所有设置
func (s *SettingService) GetSettings(category string) ([]dto.SettingVO, error) {
	settings, err := s.settingRepo.GetByCategory(category)
	if err != nil {
		return nil, err
	}

	result := make([]dto.SettingVO, 0, len(settings))
	for _, item := range settings {
		vo := dto.SettingVO{
			Key:         item.Key,
			Value:       item.Value,
			Encrypted:   item.Encrypted,
			Description: item.Description,
			UpdatedAt:   item.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		// 加密字段不返回真实值，只返回掩码
		if item.Encrypted == 1 && item.Value != "" {
			vo.Value = "******"
		}
		result = append(result, vo)
	}
	return result, nil
}

// SaveSettings 保存某分类的设置
func (s *SettingService) SaveSettings(category string, req *dto.SaveSettingsRequest, operatorID uint64) error {
	settings := make([]model.SystemSetting, 0, len(req.Settings))
	for _, item := range req.Settings {
		// 如果加密字段传来的是掩码，说明没修改，跳过
		if item.Encrypted == 1 && item.Value == "******" {
			continue
		}
		settings = append(settings, model.SystemSetting{
			Category:    category,
			Key:         item.Key,
			Value:       item.Value,
			Encrypted:   item.Encrypted,
			Description: item.Description,
			UpdatedBy:   &operatorID,
		})
	}

	if len(settings) == 0 {
		return nil
	}

	return s.settingRepo.BatchUpsert(settings)
}

// TestZentaoConnection 测试禅道连接并获取Token
// 自动尝试 /zentao/api.php/v1/tokens 和 /api.php/v1/tokens 两种路径
func (s *SettingService) TestZentaoConnection(req *dto.ZentaoTestRequest) *dto.ZentaoTestResponse {
	baseURL := strings.TrimRight(req.BaseURL, "/")
	tokenPaths := []string{"/api.php/v1/tokens", "/zentao/api.php/v1/tokens"}

	body := fmt.Sprintf(`{"account":"%s","password":"%s"}`, req.Account, req.Password)

	for _, path := range tokenPaths {
		result := s.tryZentaoToken(baseURL+path, body)
		if result.Success {
			return result
		}
	}

	return &dto.ZentaoTestResponse{Success: false, Message: "连接失败，已尝试多种API路径均无法获取Token，请检查禅道地址和账号密码"}
}

func (s *SettingService) tryZentaoToken(tokenURL, body string) *dto.ZentaoTestResponse {
	httpReq, err := http.NewRequest("POST", tokenURL, strings.NewReader(body))
	if err != nil {
		return &dto.ZentaoTestResponse{Success: false, Message: "构建请求失败: " + err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return &dto.ZentaoTestResponse{Success: false, Message: "连接失败: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &dto.ZentaoTestResponse{
			Success: false,
			Message: fmt.Sprintf("禅道返回错误 %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	// 解析token
	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return &dto.ZentaoTestResponse{
			Success: false,
			Message: "解析响应失败: " + string(respBody),
		}
	}

	if tokenResp.Token == "" {
		return &dto.ZentaoTestResponse{
			Success: false,
			Message: "未获取到Token，响应: " + string(respBody),
		}
	}

	// 连接成功，自动保存Token到数据库
	now := time.Now()
	operatorID := uint64(0)
	_ = s.settingRepo.Upsert(&model.SystemSetting{
		Category: "zentao", Key: "token", Value: tokenResp.Token, Encrypted: 1, UpdatedBy: &operatorID,
	})
	_ = s.settingRepo.Upsert(&model.SystemSetting{
		Category: "zentao", Key: "token_expire_at", Value: now.Add(24 * time.Hour).Format(time.RFC3339), UpdatedBy: &operatorID,
	})

	s.logger.Info("禅道Token获取成功", zap.String("token_url", tokenURL))

	return &dto.ZentaoTestResponse{
		Success: true,
		Token:   tokenResp.Token,
		Message: "连接成功，Token已自动保存",
	}
}

// TestGitLabConnection 测试GitLab连接
func (s *SettingService) TestGitLabConnection(req *dto.GitLabTestRequest) *dto.GitLabTestResponse {
	baseURL := strings.TrimRight(req.BaseURL, "/")
	apiURL := baseURL + "/api/v4/user"

	httpReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return &dto.GitLabTestResponse{Success: false, Message: "构建请求失败: " + err.Error()}
	}
	httpReq.Header.Set("PRIVATE-TOKEN", req.AccessToken)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return &dto.GitLabTestResponse{Success: false, Message: "连接失败: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &dto.GitLabTestResponse{
			Success: false,
			Message: fmt.Sprintf("GitLab返回错误 %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	var userResp struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	}
	_ = json.Unmarshal(respBody, &userResp)

	s.logger.Info("GitLab连接成功", zap.String("username", userResp.Username))

	return &dto.GitLabTestResponse{
		Success:  true,
		Message:  fmt.Sprintf("连接成功，当前用户: %s (%s)", userResp.Name, userResp.Username),
		Username: userResp.Username,
	}
}

// TestMailConnection 测试邮件服务连接
func (s *SettingService) TestMailConnection(req *dto.MailTestRequest) *dto.MailTestResponse {
	d := gomail.NewDialer(req.Host, req.Port, req.Username, req.Password)
	if req.UseSSL {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	conn, err := d.Dial()
	if err != nil {
		return &dto.MailTestResponse{
			Success: false,
			Message: "连接邮件服务器失败: " + err.Error(),
		}
	}
	_ = conn.Close()

	s.logger.Info("邮件服务连接成功", zap.String("host", req.Host), zap.Int("port", req.Port))
	return &dto.MailTestResponse{
		Success: true,
		Message: "邮件服务器连接成功",
	}
}

// SendTestMail 发送模板测试邮件
func (s *SettingService) SendTestMail(req *dto.SendTestMailRequest) *dto.MailTestResponse {
	cfg := LoadMailConfig()
	if strings.TrimSpace(cfg.Host) == "" {
		return &dto.MailTestResponse{
			Success: false,
			Message: "请先完成 SMTP 配置并保存",
		}
	}

	subjectKey := req.TemplateType + "_subject_template"
	bodyKey := req.TemplateType + "_body_template"
	subjectTemplate := strings.TrimSpace(req.SubjectTemplate)
	bodyTemplate := strings.TrimSpace(req.BodyTemplate)
	if subjectTemplate == "" {
		subjectTemplate = s.templateValue(subjectKey)
	}
	if bodyTemplate == "" {
		bodyTemplate = s.templateValue(bodyKey)
	}
	values := buildSampleTemplateData(req.TemplateType)
	subject := renderMailTemplate(subjectTemplate, values)
	body := renderMailTemplate(bodyTemplate, values)

	m := gomail.NewMessage()
	m.SetHeader("From", cfg.From)
	m.SetHeader("To", req.Recipient)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if cfg.UseSSL {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		return &dto.MailTestResponse{
			Success: false,
			Message: "发送测试邮件失败: " + err.Error(),
		}
	}

	s.logger.Info("模板测试邮件发送成功", zap.String("recipient", req.Recipient), zap.String("template_type", req.TemplateType))
	return &dto.MailTestResponse{
		Success: true,
		Message: "测试邮件发送成功",
	}
}

// GetZentaoToken 获取禅道Token(自动刷新)
func (s *SettingService) GetZentaoToken() (string, error) {
	token := s.settingRepo.GetValue("zentao", "token")
	expireStr := s.settingRepo.GetValue("zentao", "token_expire_at")

	// 检查是否过期
	needRefresh := token == ""
	if expireStr != "" {
		expireAt, err := time.Parse(time.RFC3339, expireStr)
		if err == nil && time.Now().After(expireAt) {
			needRefresh = true
		}
	}

	if needRefresh {
		baseURL := s.settingRepo.GetValue("zentao", "base_url")
		account := s.settingRepo.GetValue("zentao", "account")
		password := s.settingRepo.GetValue("zentao", "password")
		if baseURL == "" || account == "" || password == "" {
			return "", fmt.Errorf("禅道未配置")
		}
		resp := s.TestZentaoConnection(&dto.ZentaoTestRequest{
			BaseURL: baseURL, Account: account, Password: password,
		})
		if !resp.Success {
			return "", fmt.Errorf("刷新Token失败: %s", resp.Message)
		}
		return resp.Token, nil
	}

	return token, nil
}

// RefreshZentaoToken 强制刷新禅道 Token
func (s *SettingService) RefreshZentaoToken() (string, error) {
	baseURL := s.settingRepo.GetValue("zentao", "base_url")
	account := s.settingRepo.GetValue("zentao", "account")
	password := s.settingRepo.GetValue("zentao", "password")
	if baseURL == "" || account == "" || password == "" {
		return "", fmt.Errorf("禅道未配置")
	}

	resp := s.TestZentaoConnection(&dto.ZentaoTestRequest{
		BaseURL:  baseURL,
		Account:  account,
		Password: password,
	})
	if !resp.Success || resp.Token == "" {
		return "", fmt.Errorf("刷新Token失败: %s", resp.Message)
	}

	return resp.Token, nil
}

func (s *SettingService) templateValue(key string) string {
	value := strings.TrimSpace(s.settingRepo.GetValue("mail", key))
	if value != "" {
		return value
	}
	return defaultMailTemplate(key)
}
