package pwn

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type DisassembleTool struct {
	Timeout time.Duration
}

func NewDisassembleTool(timeout time.Duration) *DisassembleTool {
	return &DisassembleTool{Timeout: timeout}
}

func (t *DisassembleTool) Name() string {
	return "disassemble"
}

func (t *DisassembleTool) Description() string {
	return "反汇编二进制文件。可以反汇编整个文件、指定函数或指定地址范围。使用 objdump 实现。"
}

func (t *DisassembleTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "二进制文件路径",
			},
			"start_addr": map[string]any{
				"type":        "string",
				"description": "起始地址 (如 0x401000)，留空则反汇编 .text 段",
			},
			"stop_addr": map[string]any{
				"type":        "string",
				"description": "结束地址",
			},
		},
		"required": []string{"path"},
	}
}

func (t *DisassembleTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		StartAddr string `json:"start_addr"`
		StopAddr  string `json:"stop_addr"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{"-d", "--no-show-raw-insn"}
	if params.StartAddr != "" {
		args = append(args, "--start-address="+params.StartAddr)
	}
	if params.StopAddr != "" {
		args = append(args, "--stop-address="+params.StopAddr)
	}
	args = append(args, params.Path)

	cmd := exec.CommandContext(ctx, "objdump", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("objdump failed: %w", err)
	}

	result := string(output)
	// Truncate if too long
	if len(result) > 32000 {
		result = result[:32000] + "\n... [truncated]"
	}

	return result, nil
}
