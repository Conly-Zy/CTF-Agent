package llm

import (
	"context"
	"fmt"
	"io"
)

// Provider LLM 提供者接口
type Provider interface {
	CreateMessage(ctx context.Context, params MessageParams) (*MessageResponse, error)
	CreateMessageStream(ctx context.Context, params MessageParams, writer io.Writer) (*MessageResponse, error)
	Name() string
}

// ProviderType 提供者类型
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
)

// ProviderConfig 提供者配置
type ProviderConfig struct {
	Type    ProviderType `json:"type" yaml:"type"`
	APIKey  string       `json:"api_key" yaml:"api_key"`
	Model   string       `json:"model" yaml:"model"`
	BaseURL string       `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// NewProvider 创建 LLM 提供者
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderAnthropic, "":
		return NewAnthropicProvider(cfg.APIKey, cfg.Model, cfg.BaseURL)
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, cfg.BaseURL)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}
