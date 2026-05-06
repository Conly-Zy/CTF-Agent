package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Context key for auth
type contextKey string

const (
	UserIDKey contextKey = "user_id"
	APIKeyKey contextKey = "api_key"
)

// Authenticator 认证器
type Authenticator struct {
	mu         sync.RWMutex
	apiKeys    map[string]*APIKey
	jwtSecret  []byte
	enabled    bool
}

// APIKey API 密钥
type APIKey struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Active    bool      `json:"active"`
}

// NewAuthenticator 创建认证器
func NewAuthenticator(jwtSecret string, enabled bool) *Authenticator {
	return &Authenticator{
		apiKeys:   make(map[string]*APIKey),
		jwtSecret: []byte(jwtSecret),
		enabled:   enabled,
	}
}

// GenerateAPIKey 生成 API 密钥
func (a *Authenticator) GenerateAPIKey(name, userID string, expiresIn time.Duration) (*APIKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 生成随机密钥
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	key := hex.EncodeToString(bytes)

	apiKey := &APIKey{
		Key:       key,
		Name:      name,
		UserID:    userID,
		CreatedAt: time.Now(),
		Active:    true,
	}

	if expiresIn > 0 {
		apiKey.ExpiresAt = time.Now().Add(expiresIn)
	}

	a.apiKeys[key] = apiKey
	return apiKey, nil
}

// ValidateAPIKey 验证 API 密钥
func (a *Authenticator) ValidateAPIKey(key string) (*APIKey, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	apiKey, ok := a.apiKeys[key]
	if !ok {
		return nil, fmt.Errorf("invalid API key")
	}

	if !apiKey.Active {
		return nil, fmt.Errorf("API key is disabled")
	}

	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	return apiKey, nil
}

// RevokeAPIKey 撤销 API 密钥
func (a *Authenticator) RevokeAPIKey(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	apiKey, ok := a.apiKeys[key]
	if !ok {
		return fmt.Errorf("API key not found")
	}

	apiKey.Active = false
	return nil
}

// ListAPIKeys 列出所有 API 密钥
func (a *Authenticator) ListAPIKeys(userID string) []*APIKey {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var keys []*APIKey
	for _, apiKey := range a.apiKeys {
		if userID == "" || apiKey.UserID == userID {
			keys = append(keys, apiKey)
		}
	}
	return keys
}

// DeleteAPIKey 删除 API 密钥
func (a *Authenticator) DeleteAPIKey(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.apiKeys[key]; !ok {
		return fmt.Errorf("API key not found")
	}

	delete(a.apiKeys, key)
	return nil
}

// AuthMiddleware 认证中间件
func (a *Authenticator) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled {
			// 认证未启用，直接放行
			ctx := context.WithValue(r.Context(), UserIDKey, "anonymous")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 尝试从 Header 获取 API Key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// 尝试从 Authorization Header 获取
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			http.Error(w, `{"error": "API key required"}`, http.StatusUnauthorized)
			return
		}

		// 验证 API Key
		key, err := a.ValidateAPIKey(apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		// 将用户信息添加到上下文
		ctx := context.WithValue(r.Context(), UserIDKey, key.UserID)
		ctx = context.WithValue(ctx, APIKeyKey, key.Key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID 从上下文获取用户 ID
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetAPIKey 从上下文获取 API Key
func GetAPIKey(ctx context.Context) string {
	if apiKey, ok := ctx.Value(APIKeyKey).(string); ok {
		return apiKey
	}
	return ""
}

// RateLimiter 限流器
type RateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*ClientLimit
	rate     int
	burst    int
	cleanup  time.Duration
	stopCh   chan struct{}
}

// ClientLimit 客户端限制
type ClientLimit struct {
	Tokens    float64
	LastTime  time.Time
	TotalHits int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rate, burst int) *RateLimiter {
	limiter := &RateLimiter{
		clients: make(map[string]*ClientLimit),
		rate:    rate,
		burst:   burst,
		cleanup: 5 * time.Minute,
		stopCh:  make(chan struct{}),
	}

	// 启动清理协程
	go limiter.cleanupLoop()

	return limiter
}

// Allow 检查是否允许请求
func (l *RateLimiter) Allow(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	client, ok := l.clients[clientID]
	if !ok {
		client = &ClientLimit{
			Tokens:   float64(l.burst),
			LastTime: time.Now(),
		}
		l.clients[clientID] = client
	}

	// 计算时间差
	now := time.Now()
	elapsed := now.Sub(client.LastTime).Seconds()
	client.LastTime = now

	// 添加令牌
	client.Tokens += elapsed * float64(l.rate)
	if client.Tokens > float64(l.burst) {
		client.Tokens = float64(l.burst)
	}

	// 检查是否有足够的令牌
	if client.Tokens < 1 {
		return false
	}

	// 消耗令牌
	client.Tokens--
	client.TotalHits++
	return true
}

// GetClientStats 获取客户端统计
func (l *RateLimiter) GetClientStats(clientID string) map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	client, ok := l.clients[clientID]
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"tokens":     client.Tokens,
		"total_hits": client.TotalHits,
		"last_time":  client.LastTime,
	}
}

// GetAllStats 获取所有统计
func (l *RateLimiter) GetAllStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := make(map[string]interface{})
	for clientID, client := range l.clients {
		stats[clientID] = map[string]interface{}{
			"tokens":     client.Tokens,
			"total_hits": client.TotalHits,
		}
	}
	return stats
}

func (l *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanupInactive()
		case <-l.stopCh:
			return
		}
	}
}

func (l *RateLimiter) cleanupInactive() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for clientID, client := range l.clients {
		if now.Sub(client.LastTime) > l.cleanup {
			delete(l.clients, clientID)
		}
	}
}

// Stop 停止限流器
func (l *RateLimiter) Stop() {
	close(l.stopCh)
}

// RateLimitMiddleware 限流中间件
func (l *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取客户端标识（IP 或 API Key）
		clientID := r.RemoteAddr
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			clientID = apiKey
		}

		if !l.Allow(clientID) {
			http.Error(w, `{"error": "Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware CORS 中间件
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 检查是否允许该源
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// 处理预检请求
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
