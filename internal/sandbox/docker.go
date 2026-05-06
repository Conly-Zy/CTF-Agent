package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// DockerSandbox Docker 沙箱执行环境
type DockerSandbox struct {
	logger    *slog.Logger
	image     string
	workDir   string
	timeout   time.Duration
	network   bool
	privileged bool
}

func NewDockerSandbox(logger *slog.Logger, image string, timeout time.Duration) *DockerSandbox {
	return &DockerSandbox{
		logger:  logger,
		image:   image,
		timeout: timeout,
		network: true,
	}
}

// SetWorkDir 设置工作目录
func (d *DockerSandbox) SetWorkDir(dir string) {
	d.workDir = dir
}

// SetNetwork 设置是否启用网络
func (d *DockerSandbox) SetNetwork(enable bool) {
	d.network = enable
}

// SetPrivileged 设置是否启用特权模式
func (d *DockerSandbox) SetPrivileged(enable bool) {
	d.privileged = enable
}

// ExecResult 执行结果
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// Execute 在沙箱中执行命令
func (d *DockerSandbox) Execute(ctx context.Context, command string) *ExecResult {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	d.logger.Info("executing in sandbox",
		"image", d.image,
		"command", command)

	// 构建 docker run 命令
	args := []string{"run", "--rm"}

	// 添加网络配置
	if !d.network {
		args = append(args, "--network", "none")
	}

	// 添加特权模式
	if d.privileged {
		args = append(args, "--privileged")
	}

	// 添加工作目录
	if d.workDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/workspace", d.workDir))
		args = append(args, "-w", "/workspace")
	}

	// 添加镜像和命令
	args = append(args, d.image, "sh", "-c", command)

	// 执行命令
	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.Output()

	result := &ExecResult{
		Stdout: string(stdout),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = string(exitErr.Stderr)
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err
		}
	}

	d.logger.Info("sandbox execution completed",
		"exit_code", result.ExitCode,
		"stdout_len", len(result.Stdout),
		"stderr_len", len(result.Stderr))

	return result
}

// ExecuteWithFiles 在沙箱中执行命令，支持文件挂载
func (d *DockerSandbox) ExecuteWithFiles(ctx context.Context, command string, files map[string]string) *ExecResult {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	d.logger.Info("executing in sandbox with files",
		"image", d.image,
		"command", command,
		"files", len(files))

	// 构建 docker run 命令
	args := []string{"run", "--rm"}

	// 添加网络配置
	if !d.network {
		args = append(args, "--network", "none")
	}

	// 添加特权模式
	if d.privileged {
		args = append(args, "--privileged")
	}

	// 添加文件挂载
	for hostPath, containerPath := range files {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// 添加镜像和命令
	args = append(args, d.image, "sh", "-c", command)

	// 执行命令
	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.Output()

	result := &ExecResult{
		Stdout: string(stdout),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = string(exitErr.Stderr)
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err
		}
	}

	return result
}

// CheckDocker 检查 Docker 是否可用
func CheckDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	return nil
}

// PullImage 拉取 Docker 镜像
func PullImage(image string) error {
	cmd := exec.Command("docker", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %s", image, string(output))
	}
	return nil
}

// ListImages 列出本地 Docker 镜像
func ListImages() ([]string, error) {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	images := strings.Split(strings.TrimSpace(string(output)), "\n")
	return images, nil
}

// SandboxManager 沙箱管理器
type SandboxManager struct {
	logger   *slog.Logger
	sandboxes map[string]*DockerSandbox
}

func NewSandboxManager(logger *slog.Logger) *SandboxManager {
	return &SandboxManager{
		logger:    logger,
		sandboxes: make(map[string]*DockerSandbox),
	}
}

// GetOrCreate 获取或创建沙箱
func (m *SandboxManager) GetOrCreate(name string, image string, timeout time.Duration) *DockerSandbox {
	if sandbox, ok := m.sandboxes[name]; ok {
		return sandbox
	}

	sandbox := NewDockerSandbox(m.logger, image, timeout)
	m.sandboxes[name] = sandbox
	return sandbox
}

// Remove 移除沙箱
func (m *SandboxManager) Remove(name string) {
	delete(m.sandboxes, name)
}

// List 列出所有沙箱
func (m *SandboxManager) List() []string {
	names := make([]string, 0, len(m.sandboxes))
	for name := range m.sandboxes {
		names = append(names, name)
	}
	return names
}
