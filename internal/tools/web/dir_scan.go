package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type DirScanTool struct {
	Timeout time.Duration
}

func NewDirScanTool(timeout time.Duration) *DirScanTool {
	return &DirScanTool{Timeout: timeout}
}

func (t *DirScanTool) Name() string {
	return "dir_scan"
}

func (t *DirScanTool) Description() string {
	return "使用 gobuster 或 dirb 扫描 Web 目录和文件。需要目标 URL 和可选的字典路径。"
}

func (t *DirScanTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "目标 URL",
			},
			"wordlist": map[string]any{
				"type":        "string",
				"description": "字典文件路径 (默认 /usr/share/wordlists/dirb/common.txt)",
			},
			"extensions": map[string]any{
				"type":        "string",
				"description": "要扫描的文件扩展名 (如 php,txt,html)",
			},
		},
		"required": []string{"url"},
	}
}

func (t *DirScanTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		URL        string `json:"url"`
		Wordlist   string `json:"wordlist"`
		Extensions string `json:"extensions"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Wordlist == "" {
		params.Wordlist = "/usr/share/wordlists/dirb/common.txt"
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Try gobuster first, fallback to dirb
	args := []string{"dir", "-u", params.URL, "-w", params.Wordlist, "-q", "--no-color", "-t", "10"}
	if params.Extensions != "" {
		args = append(args, "-x", params.Extensions)
	}

	cmd := exec.CommandContext(ctx, "gobuster", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to dirb
		dirbArgs := []string{params.URL, params.Wordlist}
		cmd = exec.CommandContext(ctx, "dirb", dirbArgs...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("scan failed (tried gobuster and dirb): %w", err)
		}
	}

	return string(output), nil
}
