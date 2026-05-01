package reverse

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type StringsTool struct {
	Timeout time.Duration
}

func NewStringsTool(timeout time.Duration) *StringsTool {
	return &StringsTool{Timeout: timeout}
}

func (t *StringsTool) Name() string {
	return "strings_extract"
}

func (t *StringsTool) Description() string {
	return "从二进制文件中提取可打印字符串。支持设置最小长度和编码格式。"
}

func (t *StringsTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径",
			},
			"min_length": map[string]any{
				"type":        "integer",
				"description": "最小字符串长度 (默认 4)",
				"default":     4,
			},
			"encoding": map[string]any{
				"type":        "string",
				"description": "编码: ascii, utf8, utf16le (默认 ascii)",
				"default":     "ascii",
			},
		},
		"required": []string{"path"},
	}
}

func (t *StringsTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		MinLength int    `json:"min_length"`
		Encoding  string `json:"encoding"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.MinLength <= 0 {
		params.MinLength = 4
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{fmt.Sprintf("-n%d", params.MinLength)}
	switch params.Encoding {
	case "utf16le":
		args = append(args, "-el")
	case "utf8":
		args = append(args, "-eUTF-8")
	default:
		args = append(args, "-a")
	}
	args = append(args, params.Path)

	cmd := exec.CommandContext(ctx, "strings", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("strings failed: %w", err)
	}

	result := string(output)
	lines := strings.Split(result, "\n")
	if len(lines) > 1000 {
		result = strings.Join(lines[:1000], "\n") + fmt.Sprintf("\n... [truncated, %d total lines]", len(lines))
	}

	return result, nil
}
