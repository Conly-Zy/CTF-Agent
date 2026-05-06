package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// Plugin 插件定义
type Plugin struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Executable  string         `json:"executable"`
	InputSchema map[string]any `json:"input_schema"`
	Group       string         `json:"group"`
}

// PluginManifest 插件清单
type PluginManifest struct {
	Tools []Plugin `json:"tools"`
}

// PluginLoader 插件加载器
type PluginLoader struct {
	mu       sync.RWMutex
	plugins  map[string]*Plugin
	registry *tools.Registry
	dir      string
	logger   *slog.Logger
}

// NewPluginLoader 创建插件加载器
func NewPluginLoader(registry *tools.Registry, dir string, logger *slog.Logger) *PluginLoader {
	return &PluginLoader{
		plugins:  make(map[string]*Plugin),
		registry: registry,
		dir:      dir,
		logger:   logger,
	}
}

// SetRegistry 设置工具注册表
func (l *PluginLoader) SetRegistry(registry *tools.Registry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.registry = registry
}

// LoadAll 加载目录下所有插件
func (l *PluginLoader) LoadAll() error {
	if _, err := os.Stat(l.dir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(l.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == "manifest.json" {
			return l.loadManifest(path)
		}
		return nil
	})
}

func (l *PluginLoader) loadManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read manifest %s: %w", path, err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse manifest %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	for _, plugin := range manifest.Tools {
		plugin.Executable = filepath.Join(dir, plugin.Executable)
		if err := l.registerPlugin(&plugin); err != nil {
			l.logger.Warn("skip plugin", "name", plugin.Name, "error", err)
		}
	}

	return nil
}

func (l *PluginLoader) registerPlugin(p *Plugin) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.plugins[p.Name]; exists {
		return fmt.Errorf("plugin %q already registered", p.Name)
	}

	group := tools.GroupCommon
	switch p.Group {
	case "web":
		group = tools.GroupWeb
	case "pwn":
		group = tools.GroupPwn
	case "crypto":
		group = tools.GroupCrypto
	case "reverse":
		group = tools.GroupReverse
	}

	tool := &PluginTool{
		name:        p.Name,
		description: p.Description,
		executable:  p.Executable,
		schema:      p.InputSchema,
	}

	if err := l.registry.RegisterWithGroup(tool, group); err != nil {
		return err
	}

	l.plugins[p.Name] = p
	l.logger.Info("plugin loaded", "name", p.Name, "version", p.Version)
	return nil
}

// ListPlugins 列出已加载的插件
func (l *PluginLoader) ListPlugins() []Plugin {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]Plugin, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, *p)
	}
	return result
}

// Reload 重新加载所有插件
func (l *PluginLoader) Reload() error {
	l.mu.Lock()
	// Remove old plugin tools
	for name := range l.plugins {
		l.registry.Remove(name)
		delete(l.plugins, name)
	}
	l.mu.Unlock()

	return l.LoadAll()
}

// PluginTool 插件工具适配器
type PluginTool struct {
	name        string
	description string
	executable  string
	schema      map[string]any
}

func (t *PluginTool) Name() string        { return t.name }
func (t *PluginTool) Description() string  { return t.description }
func (t *PluginTool) Schema() map[string]any { return t.schema }

func (t *PluginTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	cmd := exec.CommandContext(ctx, t.executable)
	cmd.Stdin = strings.NewReader(string(input))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("plugin %s: %w\n%s", t.name, err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
