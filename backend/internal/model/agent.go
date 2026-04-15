package model

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// JSON 自定义类型，支持 GORM JSON 字段
type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("null")
		return nil
	}
	switch v := value.(type) {
	case []byte:
		// 必须复制，MySQL 驱动会复用内部缓冲区，直接引用会导致数据被后续扫描覆盖
		copied := make([]byte, len(v))
		copy(copied, v)
		*j = JSON(copied)
	case string:
		*j = JSON(v)
	}
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	normalized := bytes.TrimSpace(bytes.ToValidUTF8([]byte(j), []byte{}))
	if len(normalized) == 0 {
		return []byte("null"), nil
	}
	if json.Valid(normalized) {
		return normalized, nil
	}
	// 容错历史脏数据，避免单条记录导致整个接口响应体为空。
	return json.Marshal(string(normalized))
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = JSON(data)
	return nil
}

// Agent AI Agent
type Agent struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description   string    `gorm:"size:255;default:''" json:"description"`
	ModelProvider string    `gorm:"size:32;default:'claude'" json:"model_provider"`
	ModelName     string    `gorm:"size:64;default:''" json:"model_name"`
	APIKeyRef     string    `gorm:"size:128;default:''" json:"api_key_ref"`
	BaseURL       string    `gorm:"size:256;default:''" json:"base_url"`
	MaxTokens     int       `gorm:"default:4096" json:"max_tokens"`
	Temperature   float64   `gorm:"type:decimal(3,2);default:0.30" json:"temperature"`
	Status        int8      `gorm:"default:1" json:"status"`
	ConfigJSON    JSON      `gorm:"type:json" json:"config_json"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	// 关联
	Skills     []Skill     `gorm:"many2many:agent_skill;" json:"workflows,omitempty"`
	MCPServers []MCPServer `gorm:"many2many:agent_mcp;" json:"mcp_servers,omitempty"`
}

func (Agent) TableName() string { return "agent" }

// Skill 技能
type Skill struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description    string    `gorm:"size:255;default:''" json:"description"`
	SkillType      string    `gorm:"size:32;default:'builtin'" json:"workflow_type"`
	PromptTemplate string    `gorm:"type:text" json:"prompt_template"`
	InputSchema    JSON      `gorm:"type:json" json:"input_schema"`
	OutputSchema   JSON      `gorm:"type:json" json:"output_schema"`
	ConfigJSON     JSON      `gorm:"type:json" json:"config_json"`
	Status         int8      `gorm:"default:1" json:"status"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Skill) TableName() string { return "skill" }

// MCPServer MCP Server配置
type MCPServer struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:255;default:''" json:"description"`
	ServerType  string    `gorm:"size:32;default:'stdio'" json:"server_type"`
	Command     string    `gorm:"size:512;default:''" json:"command"`
	Args        JSON      `gorm:"type:json" json:"args"`
	URL         string    `gorm:"size:512;default:''" json:"url"`
	EnvVars     JSON      `gorm:"type:json" json:"env_vars"`
	Status      int8      `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (MCPServer) TableName() string { return "mcp_server" }

// AgentSkill Agent-Skill绑定
type AgentSkill struct {
	ID             uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID        uint64 `gorm:"uniqueIndex:uk_agent_skill;not null" json:"agent_id"`
	SkillID        uint64 `gorm:"uniqueIndex:uk_agent_skill;index;not null" json:"skill_id"`
	Priority       int    `gorm:"default:0" json:"priority"`
	ConfigOverride JSON   `gorm:"type:json" json:"config_override"`
}

func (AgentSkill) TableName() string { return "agent_skill" }

// AgentMCP Agent-MCP绑定
type AgentMCP struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID     uint64 `gorm:"uniqueIndex:uk_agent_mcp;not null" json:"agent_id"`
	MCPServerID uint64 `gorm:"uniqueIndex:uk_agent_mcp;index;not null" json:"mcp_server_id"`
}

func (AgentMCP) TableName() string { return "agent_mcp" }
