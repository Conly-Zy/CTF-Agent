package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"LLM_PROVIDER",
		"LLM_API_KEY",
		"LLM_MODEL",
		"LLM_BASE_URL",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
	} {
		t.Setenv(key, "")
	}
}

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
	clearConfigEnv(t)

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
	clearConfigEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "env-key")

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Anthropic.APIKey != "env-key" {
		t.Errorf("expected api_key 'env-key', got %q", cfg.Anthropic.APIKey)
	}
	if cfg.LLM.APIKey != "env-key" {
		t.Errorf("expected llm api_key 'env-key', got %q", cfg.LLM.APIKey)
	}
}

func TestLoadConfigLLMEnvOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("LLM_MODEL", "gpt-test")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GetProvider() != "openai" {
		t.Errorf("expected provider openai, got %q", cfg.GetProvider())
	}
	if cfg.GetAPIKey() != "openai-key" {
		t.Errorf("expected api key openai-key, got %q", cfg.GetAPIKey())
	}
	if cfg.GetModel() != "gpt-test" {
		t.Errorf("expected model gpt-test, got %q", cfg.GetModel())
	}
	if cfg.LLM.BaseURL != "https://llm.example/v1" {
		t.Errorf("expected base url override, got %q", cfg.LLM.BaseURL)
	}
}

func TestLoadConfigPlaceholderAPIKeyIsEmpty(t *testing.T) {
	clearConfigEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
llm:
  api_key: "${ANTHROPIC_API_KEY}"
anthropic:
  api_key: "${ANTHROPIC_API_KEY}"
`
	os.WriteFile(configPath, []byte(content), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GetAPIKey() != "" {
		t.Errorf("expected placeholder API key to be empty, got %q", cfg.GetAPIKey())
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
