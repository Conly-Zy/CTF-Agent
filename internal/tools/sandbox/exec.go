package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/sandbox"
)

// SandboxExecTool 在 Docker 沙箱中执行命令
type SandboxExecTool struct {
	manager *sandbox.SandboxManager
	timeout time.Duration
}

func NewSandboxExecTool(manager *sandbox.SandboxManager, timeout time.Duration) *SandboxExecTool {
	return &SandboxExecTool{
		manager: manager,
		timeout: timeout,
	}
}

func (t *SandboxExecTool) Name() string {
	return "sandbox_exec"
}

func (t *SandboxExecTool) Description() string {
	return "在隔离的 Docker 沙箱中执行命令。适用于需要安全隔离的操作。"
}

func (t *SandboxExecTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "要执行的命令",
			},
			"image": map[string]any{
				"type":        "string",
				"description": "Docker 镜像（默认: kalilinux/kali-bleeding-edge）",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "超时时间（秒，默认: 60）",
			},
		},
		"required": []string{"command"},
	}
}

func (t *SandboxExecTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
		Image   string `json:"image"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	image := params.Image
	if image == "" {
		image = "kalilinux/kali-bleeding-edge"
	}

	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}

	// 获取或创建沙箱
	sandboxName := fmt.Sprintf("sandbox-%d", time.Now().UnixNano())
	sb := t.manager.GetOrCreate(sandboxName, image, timeout)
	defer t.manager.Remove(sandboxName)

	// 执行命令
	result := sb.Execute(ctx, params.Command)

	output := fmt.Sprintf("Exit Code: %d\n", result.ExitCode)
	if result.Stdout != "" {
		output += fmt.Sprintf("STDOUT:\n%s\n", result.Stdout)
	}
	if result.Stderr != "" {
		output += fmt.Sprintf("STDERR:\n%s\n", result.Stderr)
	}
	if result.Error != nil {
		output += fmt.Sprintf("Error: %v\n", result.Error)
	}

	return output, nil
}

// SandboxFileTool 在 Docker 沙箱中操作文件
type SandboxFileTool struct {
	manager *sandbox.SandboxManager
	timeout time.Duration
}

func NewSandboxFileTool(manager *sandbox.SandboxManager, timeout time.Duration) *SandboxFileTool {
	return &SandboxFileTool{
		manager: manager,
		timeout: timeout,
	}
}

func (t *SandboxFileTool) Name() string {
	return "sandbox_file"
}

func (t *SandboxFileTool) Description() string {
	return "在 Docker 沙箱中读取或写入文件。"
}

func (t *SandboxFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "write", "list"},
				"description": "操作类型",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "写入的内容（仅 write 操作需要）",
			},
			"image": map[string]any{
				"type":        "string",
				"description": "Docker 镜像（默认: kalilinux/kali-bleeding-edge）",
			},
		},
		"required": []string{"action", "path"},
	}
}

func (t *SandboxFileTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Action  string `json:"action"`
		Path    string `json:"path"`
		Content string `json:"content"`
		Image   string `json:"image"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Action == "" || params.Path == "" {
		return "", fmt.Errorf("action and path are required")
	}

	image := params.Image
	if image == "" {
		image = "kalilinux/kali-bleeding-edge"
	}

	// 获取或创建沙箱
	sandboxName := fmt.Sprintf("sandbox-%d", time.Now().UnixNano())
	sb := t.manager.GetOrCreate(sandboxName, image, t.timeout)
	defer t.manager.Remove(sandboxName)

	var command string
	switch params.Action {
	case "read":
		command = fmt.Sprintf("cat %s", params.Path)
	case "write":
		// 写入文件
		command = fmt.Sprintf("echo '%s' > %s", params.Content, params.Path)
	case "list":
		command = fmt.Sprintf("ls -la %s", params.Path)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}

	// 执行命令
	result := sb.Execute(ctx, command)

	output := fmt.Sprintf("Exit Code: %d\n", result.ExitCode)
	if result.Stdout != "" {
		output += fmt.Sprintf("STDOUT:\n%s\n", result.Stdout)
	}
	if result.Stderr != "" {
		output += fmt.Sprintf("STDERR:\n%s\n", result.Stderr)
	}

	return output, nil
}
