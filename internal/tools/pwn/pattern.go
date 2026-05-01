package pwn

import (
	"context"
	"encoding/json"
	"fmt"
)

type PatternTool struct{}

func NewPatternTool() *PatternTool {
	return &PatternTool{}
}

func (t *PatternTool) Name() string {
	return "pattern"
}

func (t *PatternTool) Description() string {
	return "生成 De Bruijn 序列（用于确定溢出偏移量）或在序列中查找偏移量。"
}

func (t *PatternTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: generate (生成序列) 或 search (查找偏移)",
				"enum":        []string{"generate", "search"},
			},
			"length": map[string]any{
				"type":        "integer",
				"description": "生成序列的长度 (generate 时必填)",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "要搜索的值 (search 时必填，如 0x41366141 或 A6aA)",
			},
			"arch": map[string]any{
				"type":        "string",
				"description": "架构: 32 或 64 (默认 64)",
				"default":     "64",
			},
		},
		"required": []string{"action"},
	}
}

func (t *PatternTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Action string `json:"action"`
		Length int    `json:"length"`
		Value  string `json:"value"`
		Arch   string `json:"arch"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Arch == "" {
		params.Arch = "64"
	}

	switch params.Action {
	case "generate":
		if params.Length <= 0 {
			return "", fmt.Errorf("length must be positive")
		}
		pattern := generateDeBruijn(params.Length, params.Arch == "64")
		return fmt.Sprintf("Pattern (%d bytes):\n%s", len(pattern), pattern), nil

	case "search":
		if params.Value == "" {
			return "", fmt.Errorf("value is required for search")
		}
		offset := searchDeBruijn(params.Value, params.Arch == "64")
		if offset < 0 {
			return fmt.Sprintf("Value '%s' not found in pattern", params.Value), nil
		}
		return fmt.Sprintf("Offset: %d (0x%x)", offset, offset), nil

	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// De Bruijn sequence generation
const (
	lowercase = "abcdefghijklmnopqrstuvwxyz"
	uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits    = "0123456789"
)

func generateDeBruijn(length int, is64 bool) string {
	chars := lowercase + uppercase + digits
	var sb []byte
	for i := 0; i < length; i++ {
		n := i
		var c [4]byte
		for j := 0; j < 4; j++ {
			c[j] = chars[n%len(chars)]
			n /= len(chars)
		}
		if is64 {
			sb = append(sb, c[0], c[1], c[2], c[3])
		} else {
			sb = append(sb, c[0], c[1], c[2], c[3])
		}
		if len(sb) >= length {
			break
		}
	}
	return string(sb[:length])
}

func searchDeBruijn(value string, is64 bool) int {
	pattern := generateDeBruijn(20000, is64)
	// Try to find as string
	idx := indexOf(pattern, value)
	if idx >= 0 {
		return idx
	}

	// Try to interpret as hex address
	var addr uint64
	if _, err := fmt.Sscanf(value, "0x%x", &addr); err == nil {
		// Convert address to little-endian bytes
		target := []byte{
			byte(addr),
			byte(addr >> 8),
			byte(addr >> 16),
			byte(addr >> 24),
		}
		if is64 {
			target = append(target, byte(addr>>32), byte(addr>>40), byte(addr>>48), byte(addr>>56))
		}
		idx = indexOf(pattern, string(target))
		if idx >= 0 {
			return idx
		}
	}

	return -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
