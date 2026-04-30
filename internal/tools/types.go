package tools

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input json.RawMessage) (string, error)
	Schema() map[string]any
}

type ToolResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}
