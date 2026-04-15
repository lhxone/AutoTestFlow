package dto

// SettingItem 单条设置
type SettingItem struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value"`
	Encrypted   int8   `json:"encrypted"`
	Description string `json:"description"`
}

// SaveSettingsRequest 保存设置请求
type SaveSettingsRequest struct {
	Settings []SettingItem `json:"settings" binding:"required,min=1"`
}

// SettingVO 设置视图(对外隐藏加密字段的真实值)
type SettingVO struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Encrypted   int8   `json:"encrypted"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updated_at"`
}

// ZentaoTestRequest 禅道连接测试请求
type ZentaoTestRequest struct {
	BaseURL  string `json:"base_url" binding:"required"`
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ZentaoTestResponse 禅道连接测试响应
type ZentaoTestResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message"`
}

// GitLabTestRequest GitLab连接测试请求
type GitLabTestRequest struct {
	BaseURL     string `json:"base_url" binding:"required"`
	AccessToken string `json:"access_token" binding:"required"`
}

// GitLabTestResponse GitLab连接测试响应
type GitLabTestResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Username string `json:"username,omitempty"`
}

// MailTestRequest 邮件连接测试请求
type MailTestRequest struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from" binding:"required,email"`
	UseSSL   bool   `json:"use_ssl"`
}

// MailTestResponse 邮件连接测试响应
type MailTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendTestMailRequest 发送测试邮件请求
type SendTestMailRequest struct {
	Recipient       string `json:"recipient" binding:"required,email"`
	TemplateType    string `json:"template_type" binding:"required,oneof=review_result test_report"`
	SubjectTemplate string `json:"subject_template"`
	BodyTemplate    string `json:"body_template"`
}
