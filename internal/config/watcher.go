package config

import (
	"log/slog"
	"sync"
	"time"
)

// ConfigWatcher 配置文件监视器
type ConfigWatcher struct {
	configPath string
	config     *Config
	logger     *slog.Logger
	callbacks  []func(*Config)
	mu         sync.RWMutex
	stopCh     chan struct{}
	lastMod    time.Time
	interval   time.Duration
}

// NewConfigWatcher 创建配置监视器
func NewConfigWatcher(configPath string, logger *slog.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		logger:     logger,
		stopCh:     make(chan struct{}),
		interval:   5 * time.Second,
	}
}

// OnConfigChange 注册配置变更回调
func (w *ConfigWatcher) OnConfigChange(callback func(*Config)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// Start 启动监视器
func (w *ConfigWatcher) Start() {
	w.logger.Info("config watcher started", "path", w.configPath)

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkAndReload()
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop 停止监视器
func (w *ConfigWatcher) Stop() {
	close(w.stopCh)
	w.logger.Info("config watcher stopped")
}

// GetConfig 获取当前配置
func (w *ConfigWatcher) GetConfig() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

func (w *ConfigWatcher) checkAndReload() {
	// 加载新配置
	newConfig, err := Load(w.configPath)
	if err != nil {
		w.logger.Warn("failed to load config", "error", err)
		return
	}

	// 验证配置
	if err := newConfig.Validate(); err != nil {
		w.logger.Warn("invalid config", "error", err)
		return
	}

	// 检查是否有变更
	w.mu.RLock()
	oldConfig := w.config
	w.mu.RUnlock()

	if oldConfig != nil && configsEqual(oldConfig, newConfig) {
		return
	}

	// 更新配置
	w.mu.Lock()
	w.config = newConfig
	w.mu.Unlock()

	w.logger.Info("config reloaded")

	// 调用回调
	w.mu.RLock()
	callbacks := w.callbacks
	w.mu.RUnlock()

	for _, callback := range callbacks {
		callback(newConfig)
	}
}

func configsEqual(a, b *Config) bool {
	// 简单比较关键字段
	if a.Anthropic.APIKey != b.Anthropic.APIKey {
		return false
	}
	if a.Anthropic.Model != b.Anthropic.Model {
		return false
	}
	if a.Agent.MaxIterations != b.Agent.MaxIterations {
		return false
	}
	if a.Agent.Timeout != b.Agent.Timeout {
		return false
	}
	return true
}

// DynamicConfig 动态配置管理器
type DynamicConfig struct {
	mu     sync.RWMutex
	values map[string]interface{}
}

// NewDynamicConfig 创建动态配置管理器
func NewDynamicConfig() *DynamicConfig {
	return &DynamicConfig{
		values: make(map[string]interface{}),
	}
}

// Set 设置配置值
func (c *DynamicConfig) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

// Get 获取配置值
func (c *DynamicConfig) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.values[key]
	return val, ok
}

// GetString 获取字符串配置值
func (c *DynamicConfig) GetString(key string, defaultVal string) string {
	val, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	if str, ok := val.(string); ok {
		return str
	}
	return defaultVal
}

// GetInt 获取整数配置值
func (c *DynamicConfig) GetInt(key string, defaultVal int) int {
	val, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	if num, ok := val.(int); ok {
		return num
	}
	return defaultVal
}

// GetBool 获取布尔配置值
func (c *DynamicConfig) GetBool(key string, defaultVal bool) bool {
	val, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return defaultVal
}

// GetAll 获取所有配置
func (c *DynamicConfig) GetAll() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

// Delete 删除配置值
func (c *DynamicConfig) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, key)
}
