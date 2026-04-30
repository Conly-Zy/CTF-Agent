package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Anthropic AnthropicConfig `yaml:"anthropic"`
	Agent     AgentConfig     `yaml:"agent"`
	Sandbox   SandboxConfig   `yaml:"sandbox"`
	Flag      FlagConfig      `yaml:"flag"`
	Submit    SubmitConfig    `yaml:"submit"`
}

type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type AgentConfig struct {
	MaxIterations int           `yaml:"max_iterations"`
	Timeout       time.Duration `yaml:"timeout"`
	Verbose       bool          `yaml:"verbose"`
}

type SandboxConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Image      string        `yaml:"image"`
	Timeout    time.Duration `yaml:"timeout"`
	NetworkMode string       `yaml:"network_mode"`
}

type FlagConfig struct {
	Patterns []string `yaml:"patterns"`
}

type SubmitConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Method  string `yaml:"method"`
	Field   string `yaml:"field"`
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

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Anthropic.APIKey == "" {
		return fmt.Errorf("anthropic API key is required")
	}
	return nil
}
