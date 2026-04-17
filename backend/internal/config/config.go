package config

import (
	"fmt"

	"github.com/spf13/viper"
)

var envBindings = map[string]string{
	"database.password":          "ATF_DATABASE_PASSWORD",
	"database.dbname":            "ATF_DATABASE_DBNAME",
	"jwt.secret":                 "ATF_JWT_SECRET",
	"zentao.base_url":            "ATF_ZENTAO_BASE_URL",
	"zentao.api_token":           "ATF_ZENTAO_API_TOKEN",
	"git.work_dir":               "ATF_GIT_WORK_DIR",
	"cli_runtime.workspace_root": "ATF_CLI_RUNTIME_WORKSPACE_ROOT",
	"mail.host":                  "ATF_MAIL_HOST",
	"mail.port":                  "ATF_MAIL_PORT",
	"mail.username":              "ATF_MAIL_USERNAME",
	"mail.password":              "ATF_MAIL_PASSWORD",
	"mail.from":                  "ATF_MAIL_FROM",
}

// AppConfig 全局应用配置
type AppConfig struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Zentao     ZentaoConfig     `mapstructure:"zentao"`
	AI         AIConfig         `mapstructure:"ai"`
	Git        GitConfig        `mapstructure:"git"`
	CLIRuntime CLIRuntimeConfig `mapstructure:"cli_runtime"`
	Mail       MailConfig       `mapstructure:"mail"`
	Log        LogConfig        `mapstructure:"log"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

type JWTConfig struct {
	Secret             string `mapstructure:"secret"`
	ExpireHours        int    `mapstructure:"expire_hours"`
	RefreshExpireHours int    `mapstructure:"refresh_expire_hours"`
}

type ZentaoConfig struct {
	BaseURL      string `mapstructure:"base_url"`
	APIToken     string `mapstructure:"api_token"`
	SyncInterval string `mapstructure:"sync_interval"`
}

type AIConfig struct {
	Provider    string  `mapstructure:"provider"`
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type GitConfig struct {
	WorkDir      string `mapstructure:"work_dir"`
	CommitAuthor string `mapstructure:"commit_author"`
	CommitEmail  string `mapstructure:"commit_email"`
}

type CLIRuntimeConfig struct {
	Command           string            `mapstructure:"command"`
	Args              []string          `mapstructure:"args"`
	Timeout           string            `mapstructure:"timeout"`
	WorkspaceRoot     string            `mapstructure:"workspace_root"`
	RepoDirName       string            `mapstructure:"repo_dir_name"`
	ControlDirName    string            `mapstructure:"control_dir_name"`
	InputFileName     string            `mapstructure:"input_file_name"`
	PromptFileName    string            `mapstructure:"prompt_file_name"`
	ResultFileName    string            `mapstructure:"result_file_name"`
	LogFileName       string            `mapstructure:"log_file_name"`
	PreserveWorkspace bool              `mapstructure:"preserve_workspace"`
	Env               map[string]string `mapstructure:"env"`
}

type MailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	UseSSL   bool   `mapstructure:"use_ssl"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

// Global 全局配置实例
var Global *AppConfig

// Load 加载配置文件
func Load(path string) (*AppConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	for key, envName := range envBindings {
		if err := viper.BindEnv(key, envName); err != nil {
			return nil, fmt.Errorf("绑定环境变量失败 %s: %w", envName, err)
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &AppConfig{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	Global = cfg
	return cfg, nil
}
