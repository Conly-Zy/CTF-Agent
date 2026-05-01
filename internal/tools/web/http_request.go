package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPRequestTool struct {
	Timeout time.Duration
}

func NewHTTPRequestTool(timeout time.Duration) *HTTPRequestTool {
	return &HTTPRequestTool{Timeout: timeout}
}

func (t *HTTPRequestTool) Name() string {
	return "http_request"
}

func (t *HTTPRequestTool) Description() string {
	return "发送 HTTP 请求并返回响应（状态码、头部、正文）。用于探测 Web 服务、测试 API 端点、发送 payload。"
}

func (t *HTTPRequestTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "请求 URL",
			},
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP 方法 (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)",
				"default":     "GET",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "请求头 (键值对)",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "请求体内容",
			},
		},
		"required": []string{"url"},
	}
}

func (t *HTTPRequestTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Method == "" {
		params.Method = "GET"
	}
	params.Method = strings.ToUpper(params.Method)

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if params.Body != "" {
		bodyReader = strings.NewReader(params.Body)
	}

	req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	for k, v := range params.Headers {
		req.Header.Set(k, v)
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "CTF-Agent/1.0")
	}

	client := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limit to 64KB)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// Build structured output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP/%d.%d %d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status))
	for k, v := range resp.Header {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString(string(respBody))

	if len(respBody) == 64*1024 {
		sb.WriteString("\n... [truncated at 64KB]")
	}

	return sb.String(), nil
}
