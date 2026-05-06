package alerting

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertError    AlertLevel = "error"
	AlertCritical AlertLevel = "critical"
)

// Alert 告警
type Alert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Source    string     `json:"source"`
	Timestamp time.Time  `json:"timestamp"`
	Resolved  bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string
	Condition func() bool
	Level     AlertLevel
	Title     string
	Message   func() string
	Interval  time.Duration
}

// AlertManager 告警管理器
type AlertManager struct {
	mu          sync.RWMutex
	alerts      map[string]*Alert
	rules       []AlertRule
	logger      *slog.Logger
	callbacks   []func(Alert)
	stopCh      chan struct{}
	nextID      int
}

// NewAlertManager 创建告警管理器
func NewAlertManager(logger *slog.Logger) *AlertManager {
	return &AlertManager{
		alerts: make(map[string]*Alert),
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// OnAlert 注册告警回调
func (m *AlertManager) OnAlert(callback func(Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// AddRule 添加告警规则
func (m *AlertManager) AddRule(rule AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// Start 启动告警管理器
func (m *AlertManager) Start() {
	m.logger.Info("alert manager started")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.evaluateRules()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop 停止告警管理器
func (m *AlertManager) Stop() {
	close(m.stopCh)
	m.logger.Info("alert manager stopped")
}

// Trigger 触发告警
func (m *AlertManager) Trigger(level AlertLevel, title, message, source string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	alert := &Alert{
		ID:        fmt.Sprintf("alert-%d", m.nextID),
		Level:     level,
		Title:     title,
		Message:   message,
		Source:    source,
		Timestamp: time.Now(),
	}

	m.alerts[alert.ID] = alert

	m.logger.Warn("alert triggered",
		"id", alert.ID,
		"level", level,
		"title", title)

	// 调用回调
	callbacks := m.callbacks
	go func() {
		for _, callback := range callbacks {
			callback(*alert)
		}
	}()

	return alert.ID
}

// Resolve 解决告警
func (m *AlertManager) Resolve(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	if alert.Resolved {
		return fmt.Errorf("alert already resolved: %s", alertID)
	}

	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now

	m.logger.Info("alert resolved", "id", alertID)
	return nil
}

// GetAlert 获取告警
func (m *AlertManager) GetAlert(alertID string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}

	return alert, nil
}

// GetActiveAlerts 获取活跃告警
func (m *AlertManager) GetActiveAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []Alert
	for _, alert := range m.alerts {
		if !alert.Resolved {
			alerts = append(alerts, *alert)
		}
	}
	return alerts
}

// GetAlertsByLevel 按级别获取告警
func (m *AlertManager) GetAlertsByLevel(level AlertLevel) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []Alert
	for _, alert := range m.alerts {
		if alert.Level == level {
			alerts = append(alerts, *alert)
		}
	}
	return alerts
}

// GetAllAlerts 获取所有告警
func (m *AlertManager) GetAllAlerts(limit int) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]Alert, 0, len(m.alerts))
	for _, alert := range m.alerts {
		alerts = append(alerts, *alert)
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(alerts)-1; i++ {
		for j := i + 1; j < len(alerts); j++ {
			if alerts[i].Timestamp.Before(alerts[j].Timestamp) {
				alerts[i], alerts[j] = alerts[j], alerts[i]
			}
		}
	}

	if limit > 0 && limit < len(alerts) {
		alerts = alerts[:limit]
	}

	return alerts
}

// GetStats 获取告警统计
func (m *AlertManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total":    len(m.alerts),
		"active":   0,
		"resolved": 0,
		"levels":   make(map[AlertLevel]int),
	}

	levelCounts := stats["levels"].(map[AlertLevel]int)
	for _, alert := range m.alerts {
		if alert.Resolved {
			stats["resolved"] = stats["resolved"].(int) + 1
		} else {
			stats["active"] = stats["active"].(int) + 1
		}
		levelCounts[alert.Level]++
	}

	return stats
}

func (m *AlertManager) evaluateRules() {
	m.mu.RLock()
	rules := m.rules
	m.mu.RUnlock()

	for _, rule := range rules {
		if rule.Condition() {
			message := ""
			if rule.Message != nil {
				message = rule.Message()
			}
			m.Trigger(rule.Level, rule.Title, message, rule.Name)
		}
	}
}

// DefaultRules 默认告警规则
func DefaultRules() []AlertRule {
	return []AlertRule{
		{
			Name: "high_error_rate",
			Condition: func() bool {
				// 这里需要访问指标收集器
				return false
			},
			Level:   AlertWarning,
			Title:   "高错误率",
			Message: func() string { return "系统错误率超过阈值" },
			Interval: 5 * time.Minute,
		},
		{
			Name: "agent_timeout",
			Condition: func() bool {
				return false
			},
			Level:   AlertError,
			Title:   "Agent 超时",
			Message: func() string { return "Agent 执行超时" },
			Interval: 1 * time.Minute,
		},
		{
			Name: "memory_usage",
			Condition: func() bool {
				return false
			},
			Level:   AlertWarning,
			Title:   "内存使用过高",
			Message: func() string { return "系统内存使用率超过 80%" },
			Interval: 5 * time.Minute,
		},
	}
}
