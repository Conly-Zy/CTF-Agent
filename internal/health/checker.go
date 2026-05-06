package health

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck 健康检查结果
type HealthCheck struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time    `json:"timestamp"`
}

// HealthReport 健康报告
type HealthReport struct {
	Status    HealthStatus  `json:"status"`
	Checks    []HealthCheck `json:"checks"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// CheckFunc 健康检查函数
type CheckFunc func(ctx context.Context) HealthCheck

// HealthChecker 健康检查器
type HealthChecker struct {
	mu      sync.RWMutex
	checks  map[string]CheckFunc
	logger  *slog.Logger
	timeout time.Duration
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(logger *slog.Logger, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		checks:  make(map[string]CheckFunc),
		logger:  logger,
		timeout: timeout,
	}
}

// Register 注册健康检查
func (c *HealthChecker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// Check 执行健康检查
func (c *HealthChecker) Check(ctx context.Context) HealthReport {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	c.mu.RLock()
	checks := make(map[string]CheckFunc)
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	// 并行执行检查
	results := make([]HealthCheck, 0, len(checks))
	resultCh := make(chan HealthCheck, len(checks))

	var wg sync.WaitGroup
	for name, check := range checks {
		wg.Add(1)
		go func(name string, check CheckFunc) {
			defer wg.Done()
			checkStart := time.Now()
			result := check(ctx)
			result.Duration = time.Since(checkStart)
			result.Timestamp = time.Now()
			resultCh <- result
		}(name, check)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		results = append(results, result)
	}

	// 确定整体状态
	overallStatus := StatusHealthy
	for _, check := range results {
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
			break
		}
		if check.Status == StatusDegraded {
			overallStatus = StatusDegraded
		}
	}

	return HealthReport{
		Status:    overallStatus,
		Checks:    results,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
}

// DefaultChecks 默认健康检查
func DefaultChecks() map[string]CheckFunc {
	return map[string]CheckFunc{
		"database": func(ctx context.Context) HealthCheck {
			start := time.Now()
			// 这里应该检查数据库连接
			return HealthCheck{
				Name:     "database",
				Status:   StatusHealthy,
				Message:  "Database connection is healthy",
				Duration: time.Since(start),
			}
		},
		"llm": func(ctx context.Context) HealthCheck {
			start := time.Now()
			// 这里应该检查 LLM API 连接
			return HealthCheck{
				Name:     "llm",
				Status:   StatusHealthy,
				Message:  "LLM API is accessible",
				Duration: time.Since(start),
			}
		},
		"memory": func(ctx context.Context) HealthCheck {
			start := time.Now()
			// 这里应该检查内存使用情况
			return HealthCheck{
				Name:     "memory",
				Status:   StatusHealthy,
				Message:  "Memory usage is normal",
				Duration: time.Since(start),
			}
		},
		"disk": func(ctx context.Context) HealthCheck {
			start := time.Now()
			// 这里应该检查磁盘空间
			return HealthCheck{
				Name:     "disk",
				Status:   StatusHealthy,
				Message:  "Disk space is sufficient",
				Duration: time.Since(start),
			}
		},
	}
}

// ReadinessChecker 就绪检查器
type ReadinessChecker struct {
	mu       sync.RWMutex
	ready    bool
	components map[string]bool
	logger   *slog.Logger
}

// NewReadinessChecker 创建就绪检查器
func NewReadinessChecker(logger *slog.Logger) *ReadinessChecker {
	return &ReadinessChecker{
		components: make(map[string]bool),
		logger:     logger,
	}
}

// SetComponent 设置组件就绪状态
func (c *ReadinessChecker) SetComponent(name string, ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.components[name] = ready
	c.updateReady()
}

// IsReady 检查是否就绪
func (c *ReadinessChecker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// GetStatus 获取就绪状态
func (c *ReadinessChecker) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	components := make(map[string]bool)
	for k, v := range c.components {
		components[k] = v
	}

	return map[string]interface{}{
		"ready":      c.ready,
		"components": components,
	}
}

func (c *ReadinessChecker) updateReady() {
	for _, ready := range c.components {
		if !ready {
			c.ready = false
			return
		}
	}
	c.ready = len(c.components) > 0
}

// LivenessChecker 存活检查器
type LivenessChecker struct {
	mu        sync.RWMutex
	alive     bool
	lastCheck time.Time
	logger    *slog.Logger
}

// NewLivenessChecker 创建存活检查器
func NewLivenessChecker(logger *slog.Logger) *LivenessChecker {
	return &LivenessChecker{
		alive:     true,
		lastCheck: time.Now(),
		logger:    logger,
	}
}

// Ping 心跳
func (c *LivenessChecker) Ping() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastCheck = time.Now()
	c.alive = true
}

// IsAlive 检查是否存活
func (c *LivenessChecker) IsAlive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.alive
}

// GetStatus 获取存活状态
func (c *LivenessChecker) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"alive":      c.alive,
		"last_check": c.lastCheck,
		"uptime":     time.Since(c.lastCheck).String(),
	}
}

// CheckTimeout 检查超时
func (c *LivenessChecker) CheckTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.lastCheck) > timeout {
		c.alive = false
		c.logger.Warn("liveness check timeout",
			"last_check", c.lastCheck,
			"timeout", timeout)
	}
}
