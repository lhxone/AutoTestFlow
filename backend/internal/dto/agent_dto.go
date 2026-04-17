package dto

import "encoding/json"

// CreateAgentRequest 创建Agent请求
type CreateAgentRequest struct {
	Name          string          `json:"name" binding:"required,max=64"`
	Description   string          `json:"description"`
	IsDefault     bool            `json:"is_default"`
	ModelProvider string          `json:"model_provider" binding:"required,oneof=claude openai zhipu custom"`
	ModelName     string          `json:"model_name" binding:"required"`
	APIKeyRef     string          `json:"api_key_ref"`
	BaseURL       string          `json:"base_url"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   float64         `json:"temperature"`
	ConfigJSON    json.RawMessage `json:"config_json"`
	WorkflowIDs   []uint64        `json:"workflow_ids"`
	MCPServerIDs  []uint64        `json:"mcp_server_ids"`
}

// UpdateAgentRequest 更新Agent请求
type UpdateAgentRequest struct {
	Name          string          `json:"name" binding:"max=64"`
	Description   string          `json:"description"`
	IsDefault     *bool           `json:"is_default"`
	ModelProvider string          `json:"model_provider" binding:"omitempty,oneof=claude openai zhipu custom"`
	ModelName     string          `json:"model_name"`
	APIKeyRef     string          `json:"api_key_ref"`
	BaseURL       string          `json:"base_url"`
	MaxTokens     *int            `json:"max_tokens"`
	Temperature   *float64        `json:"temperature"`
	Status        *int8           `json:"status"`
	ConfigJSON    json.RawMessage `json:"config_json"`
	WorkflowIDs   *[]uint64       `json:"workflow_ids"`
	MCPServerIDs  *[]uint64       `json:"mcp_server_ids"`
}

// AgentListQuery Agent列表查询
type AgentListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   *int8  `form:"status"`
}

type TestAgentConnectionRequest struct {
	ModelProvider string  `json:"model_provider" binding:"required,oneof=claude openai zhipu custom"`
	ModelName     string  `json:"model_name" binding:"required"`
	APIKeyRef     string  `json:"api_key_ref"`
	TestAPIKey    string  `json:"test_api_key"`
	BaseURL       string  `json:"base_url"`
	MaxTokens     int     `json:"max_tokens"`
	Temperature   float64 `json:"temperature"`
}

type TestAgentConnectionResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url"`
	LatencyMs    int64  `json:"latency_ms"`
	SampleOutput string `json:"sample_output,omitempty"`
}

// CreateWorkflowRequest 创建Workflow请求
type CreateWorkflowRequest struct {
	Name           string          `json:"name" binding:"required,max=64"`
	Description    string          `json:"description"`
	WorkflowType   string          `json:"workflow_type" binding:"oneof=builtin custom"`
	PromptTemplate string          `json:"prompt_template"`
	InputSchema    json.RawMessage `json:"input_schema"`
	OutputSchema   json.RawMessage `json:"output_schema"`
	ConfigJSON     json.RawMessage `json:"config_json"`
}

// UpdateWorkflowRequest 更新Workflow请求
type UpdateWorkflowRequest struct {
	Name           string          `json:"name" binding:"omitempty,max=64"`
	Description    string          `json:"description"`
	WorkflowType   string          `json:"workflow_type" binding:"omitempty,oneof=builtin custom"`
	PromptTemplate string          `json:"prompt_template"`
	InputSchema    json.RawMessage `json:"input_schema"`
	OutputSchema   json.RawMessage `json:"output_schema"`
	ConfigJSON     json.RawMessage `json:"config_json"`
	Status         *int8           `json:"status"`
}

// CreateMCPServerRequest 创建MCP Server请求
type CreateMCPServerRequest struct {
	Name        string          `json:"name" binding:"required,max=64"`
	Description string          `json:"description"`
	ServerType  string          `json:"server_type" binding:"required,oneof=stdio sse streamable_http"`
	Command     string          `json:"command"`
	Args        json.RawMessage `json:"args"`
	URL         string          `json:"url"`
	EnvVars     json.RawMessage `json:"env_vars"`
}

// UpdateMCPServerRequest 更新 MCP Server 请求
type UpdateMCPServerRequest struct {
	Name        string           `json:"name" binding:"omitempty,max=64"`
	Description string           `json:"description"`
	ServerType  string           `json:"server_type" binding:"omitempty,oneof=stdio sse streamable_http"`
	Command     string           `json:"command"`
	Args        *json.RawMessage `json:"args"`
	URL         string           `json:"url"`
	EnvVars     *json.RawMessage `json:"env_vars"`
	Status      *int8            `json:"status"`
}
