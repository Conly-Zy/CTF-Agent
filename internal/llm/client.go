package llm

import (
	"context"
	"io"
)

// Client LLM 客户端（委托给 Provider）
type Client struct {
	provider Provider
}

func NewClient(apiKey string, model string) (*Client, error) {
	provider, err := NewAnthropicProvider(apiKey, model, "")
	if err != nil {
		return nil, err
	}
	return &Client{provider: provider}, nil
}

// NewClientWithProvider 使用指定提供者创建客户端
func NewClientWithProvider(provider Provider) *Client {
	return &Client{provider: provider}
}

func (c *Client) CreateMessage(ctx context.Context, params MessageParams) (*MessageResponse, error) {
	return c.provider.CreateMessage(ctx, params)
}

func (c *Client) CreateMessageStream(ctx context.Context, params MessageParams, writer io.Writer) (*MessageResponse, error) {
	return c.provider.CreateMessageStream(ctx, params, writer)
}

// ProviderName 返回当前提供者名称
func (c *Client) ProviderName() string {
	return c.provider.Name()
}
