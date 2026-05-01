package pwn

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type BinaryInfoTool struct {
	Timeout time.Duration
}

func NewBinaryInfoTool(timeout time.Duration) *BinaryInfoTool {
	return &BinaryInfoTool{Timeout: timeout}
}

func (t *BinaryInfoTool) Name() string {
	return "binary_info"
}

func (t *BinaryInfoTool) Description() string {
	return "分析二进制文件的基本信息：文件类型、架构、保护机制（RELRO/Canary/NX/PIE）、动态库依赖。"
}

func (t *BinaryInfoTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "二进制文件路径",
			},
		},
		"required": []string{"path"},
	}
}

func (t *BinaryInfoTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	var sb strings.Builder

	// file command
	ctx1, cancel1 := context.WithTimeout(ctx, timeout)
	defer cancel1()
	if out, err := exec.CommandContext(ctx1, "file", params.Path).CombinedOutput(); err == nil {
		sb.WriteString("=== 文件类型 ===\n")
		sb.WriteString(string(out))
		sb.WriteString("\n")
	}

	// checksec (from pwntools)
	ctx2, cancel2 := context.WithTimeout(ctx, timeout)
	defer cancel2()
	if out, err := exec.CommandContext(ctx2, "checksec", "--file="+params.Path).CombinedOutput(); err == nil {
		sb.WriteString("=== 安全机制 ===\n")
		sb.WriteString(string(out))
		sb.WriteString("\n")
	} else {
		// Fallback: readelf
		ctx3, cancel3 := context.WithTimeout(ctx, timeout)
		defer cancel3()
		if out, err := exec.CommandContext(ctx3, "readelf", "-l", params.Path).CombinedOutput(); err == nil {
			sb.WriteString("=== ELF 程序头 ===\n")
			sb.WriteString(string(out))
			sb.WriteString("\n")
		}
	}

	// readelf -s (symbols)
	ctx4, cancel4 := context.WithTimeout(ctx, timeout)
	defer cancel4()
	if out, err := exec.CommandContext(ctx4, "readelf", "-s", params.Path).CombinedOutput(); err == nil {
		sb.WriteString("=== 符号表 ===\n")
		sb.WriteString(string(out))
		sb.WriteString("\n")
	}

	// ldd (dynamic libraries)
	ctx5, cancel5 := context.WithTimeout(ctx, timeout)
	defer cancel5()
	if out, err := exec.CommandContext(ctx5, "ldd", params.Path).CombinedOutput(); err == nil {
		sb.WriteString("=== 动态库依赖 ===\n")
		sb.WriteString(string(out))
	}

	return sb.String(), nil
}
