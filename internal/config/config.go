package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM       LLMConfig       `yaml:"llm" json:"llm"`
	Anthropic AnthropicConfig `yaml:"anthropic" json:"anthropic"`
	Agent     AgentConfig     `yaml:"agent" json:"agent"`
	Sandbox   SandboxConfig   `yaml:"sandbox" json:"sandbox"`
	Flag      FlagConfig      `yaml:"flag" json:"flag"`
	Submit    SubmitConfig    `yaml:"submit" json:"submit"`
	Auth      AuthConfig      `yaml:"auth" json:"auth"`
	path      string          `yaml:"-" json:"-"`
}

// LLMConfig 多提供者配置
type LLMConfig struct {
	Provider string `yaml:"provider" json:"provider"` // anthropic, openai
	APIKey   string `yaml:"api_key" json:"api_key"`
	Model    string `yaml:"model" json:"model"`
	BaseURL  string `yaml:"base_url" json:"base_url,omitempty"`
}

type AnthropicConfig struct {
	APIKey string `yaml:"api_key" json:"api_key"`
	Model  string `yaml:"model" json:"model"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	APIKey    string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	RateLimit int    `yaml:"rate_limit" json:"rate_limit"` // 每秒请求数
	RateBurst int    `yaml:"rate_burst" json:"rate_burst"` // 突发容量
}

type AgentConfig struct {
	MaxIterations int           `yaml:"max_iterations" json:"max_iterations"`
	Timeout       time.Duration `yaml:"timeout" json:"-"`
	TimeoutSec    int           `yaml:"-" json:"timeout_seconds"`
	Verbose       bool          `yaml:"verbose" json:"verbose"`
}

type SandboxConfig struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Image       string        `yaml:"image" json:"image"`
	Timeout     time.Duration `yaml:"timeout" json:"-"`
	TimeoutSec  int           `yaml:"-" json:"timeout_seconds"`
	NetworkMode string        `yaml:"network_mode" json:"network_mode"`
}

type FlagConfig struct {
	Patterns []string `yaml:"patterns" json:"patterns"`
}

type SubmitConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	URL     string `yaml:"url" json:"url"`
	Method  string `yaml:"method" json:"method"`
	Field   string `yaml:"field" json:"field"`
}

func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: "anthropic",
			Model:    "claude-opus-4-7",
		},
		Anthropic: AnthropicConfig{
			Model: "claude-opus-4-7",
		},
		Agent: AgentConfig{
			MaxIterations: 50,
			Timeout:       10 * time.Minute,
			Verbose:       false,
		},
		Sandbox: SandboxConfig{
			Enabled:     true,
			Image:       "ctf-agent-sandbox:latest",
			Timeout:     60 * time.Second,
			NetworkMode: "bridge",
		},
		Flag: FlagConfig{
			Patterns: []string{
				`flag\{[^}]+\}`,
				`CTF\{[^}]+\}`,
				`picoCTF\{[^}]+\}`,
				`FLAG\{[^}]+\}`,
			},
		},
		Submit: SubmitConfig{
			Method: "POST",
			Field:  "flag",
		},
		Auth: AuthConfig{
			Enabled:   false,
			RateLimit: 10,
			RateBurst: 20,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	cfg.normalizePlaceholders()
	cfg.applyEnvOverrides()
	cfg.SyncTimeoutSec()
	return cfg, nil
}

func (c *Config) normalizePlaceholders() {
	if isEnvPlaceholder(c.LLM.APIKey) {
		c.LLM.APIKey = ""
	}
	if isEnvPlaceholder(c.Anthropic.APIKey) {
		c.Anthropic.APIKey = ""
	}
}

func isEnvPlaceholder(value string) bool {
	return len(value) > 3 && value[:2] == "${" && value[len(value)-1:] == "}"
}

func (c *Config) applyEnvOverrides() {
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		c.LLM.Provider = provider
	}
	if model := os.Getenv("LLM_MODEL"); model != "" {
		c.LLM.Model = model
		if c.GetProvider() == "anthropic" {
			c.Anthropic.Model = model
		}
	}
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		c.LLM.BaseURL = baseURL
	}

	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		c.Anthropic.APIKey = envKey
		if c.GetProvider() == "anthropic" {
			c.LLM.APIKey = envKey
		}
	}
	if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" && c.GetProvider() == "openai" {
		c.LLM.APIKey = envKey
	}
	if envKey := os.Getenv("LLM_API_KEY"); envKey != "" {
		c.LLM.APIKey = envKey
	}
}

func (c *Config) SyncTimeoutSec() {
	c.Agent.TimeoutSec = int(c.Agent.Timeout.Seconds())
	c.Sandbox.TimeoutSec = int(c.Sandbox.Timeout.Seconds())
}

func (c *Config) applyTimeoutSec() {
	if c.Agent.TimeoutSec > 0 {
		c.Agent.Timeout = time.Duration(c.Agent.TimeoutSec) * time.Second
	}
	if c.Sandbox.TimeoutSec > 0 {
		c.Sandbox.Timeout = time.Duration(c.Sandbox.TimeoutSec) * time.Second
	}
}

func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("no config path set")
	}
	c.applyTimeoutSec()
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(c.path, data, 0644)
}

func (c *Config) Validate() error {
	if c.GetAPIKey() == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

// GetAPIKey 获取有效的 API Key（优先 LLM 配置，回退到 Anthropic 配置）
func (c *Config) GetAPIKey() string {
	if c.LLM.APIKey != "" {
		return c.LLM.APIKey
	}
	return c.Anthropic.APIKey
}

// GetModel 获取有效的模型名称
func (c *Config) GetModel() string {
	if c.LLM.Model != "" {
		return c.LLM.Model
	}
	if c.Anthropic.Model != "" {
		return c.Anthropic.Model
	}
	return "claude-opus-4-7"
}

// GetProvider 获取 LLM 提供者类型
func (c *Config) GetProvider() string {
	if c.LLM.Provider != "" {
		return c.LLM.Provider
	}
	return "anthropic"
}
