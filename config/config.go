package config

import (
	"time"

	"github.com/prometheus/common/model"
)

// Config 告警引擎配置
type Config struct {
	// 告警通知重试次数
	NotifyRetries int `yaml:"notify_retries" json:"notify_retries"`

	// 网关服务配置
	Gateway GatewayConfig `yaml:"gateway" json:"gateway"`

	// 规则评估间隔 (如: 30s)
	EvaluationInterval model.Duration `yaml:"evaluation_interval" json:"evaluation_interval"`

	// 规则重载间隔 (如: 5m)
	ReloadInterval model.Duration `yaml:"reload_interval" json:"reload_interval"`

	// 认证Token
	AuthToken string `yaml:"auth_token" json:"auth_token"`

	// 规则存储配置
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// 日志配置
	Log LogConfig `yaml:"log" json:"log"`

	// 指标暴露端口
	MetricsPort int `yaml:"metrics_port" json:"metrics_port"`

	// 是否开启告警通知
	EnableNotify bool `yaml:"enable_notify" json:"enable_notify"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	// 网关基础URL
	URL string `yaml:"url" json:"url"`

	// 规则列表路径
	RulePath string `yaml:"rule_path" json:"rule_path"`

	// 数据源列表路径
	PromPath string `yaml:"prom_path" json:"prom_path"`

	// 告警通知路径
	NotifyPath string `yaml:"notify_path" json:"notify_path"`

	// 请求超时时间
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	// 规则文件存储目录
	RuleDir string `yaml:"rule_dir" json:"rule_dir"`

	// 规则历史保留天数
	RetentionDays int `yaml:"retention_days" json:"retention_days"`

	// 是否启用历史记录
	EnableHistory bool `yaml:"enable_history" json:"enable_history"`
}

// LogConfig 日志配置
type LogConfig struct {
	// 日志级别: debug, info, warn, error
	Level string `yaml:"level" json:"level"`

	// 日志格式: json, console
	Format string `yaml:"format" json:"format"`

	// 日志输出路径
	OutputPath string `yaml:"output_path" json:"output_path"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		NotifyRetries: 3,
		Gateway: GatewayConfig{
			URL:        "http://localhost:32002",
			RulePath:   "/api/v1/rules",
			PromPath:   "/api/v1/proms",
			NotifyPath: "/api/v1/alerts",
			Timeout:    10 * time.Second,
		},
		EvaluationInterval: model.Duration(30 * time.Second),
		ReloadInterval:     model.Duration(5 * time.Minute),
		Storage: StorageConfig{
			RuleDir:       "/var/lib/alertengine/rules",
			RetentionDays: 30,
			EnableHistory: true,
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "/var/log/alertengine/alertengine.log",
		},
		MetricsPort: 9090,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Gateway.URL == "" {
		return ErrInvalidConfig("gateway.url cannot be empty")
	}
	if c.EvaluationInterval <= 0 {
		return ErrInvalidConfig("evaluation_interval must be positive")
	}
	if c.ReloadInterval <= 0 {
		return ErrInvalidConfig("reload_interval must be positive")
	}
	if c.Storage.RuleDir == "" {
		return ErrInvalidConfig("storage.rule_dir cannot be empty")
	}
	return nil
}
