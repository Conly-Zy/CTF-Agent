package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Anthropic AnthropicConfig `yaml:"anthropic" json:"anthropic"`
	Agent     AgentConfig     `yaml:"agent" json:"agent"`
	Sandbox   SandboxConfig   `yaml:"sandbox" json:"sandbox"`
	Flag      FlagConfig      `yaml:"flag" json:"flag"`
	Submit    SubmitConfig    `yaml:"submit" json:"submit"`
	path      string          `yaml:"-" json:"-"`
}

type AnthropicConfig struct {
	APIKey string `yaml:"api_key" json:"api_key"`
	Model  string `yaml:"model" json:"model"`
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

	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		cfg.Anthropic.APIKey = envKey
	}

	cfg.SyncTimeoutSec()
	return cfg, nil
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
	if c.Anthropic.APIKey == "" {
		return fmt.Errorf("anthropic API key is required")
	}
	return nil
}
