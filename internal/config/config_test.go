package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Anthropic.Model != "claude-opus-4-7" {
		t.Errorf("expected model 'claude-opus-4-7', got %q", cfg.Anthropic.Model)
	}

	if cfg.Agent.MaxIterations != 50 {
		t.Errorf("expected max_iterations 50, got %d", cfg.Agent.MaxIterations)
	}

	if !cfg.Sandbox.Enabled {
		t.Error("expected sandbox enabled by default")
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
anthropic:
  api_key: "test-key"
  model: "claude-sonnet-4-6"
agent:
  max_iterations: 25
`
	os.WriteFile(configPath, []byte(content), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Anthropic.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", cfg.Anthropic.APIKey)
	}

	if cfg.Anthropic.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model 'claude-sonnet-4-6', got %q", cfg.Anthropic.Model)
	}

	if cfg.Agent.MaxIterations != 25 {
		t.Errorf("expected max_iterations 25, got %d", cfg.Agent.MaxIterations)
	}
}

func TestLoadConfigEnvVar(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "env-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Anthropic.APIKey != "env-key" {
		t.Errorf("expected api_key 'env-key', got %q", cfg.Anthropic.APIKey)
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for missing API key")
	}

	cfg.Anthropic.APIKey = "test-key"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
