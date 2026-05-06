package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Conly-Zy/CTF-Agent/internal/memory"
)

// MemorySearchTool 语义搜索记忆
type MemorySearchTool struct {
	memoryMgr *memory.MemoryManager
}

func NewMemorySearchTool(memoryMgr *memory.MemoryManager) *MemorySearchTool {
	return &MemorySearchTool{memoryMgr: memoryMgr}
}

func (t *MemorySearchTool) Name() string {
	return "memory_search"
}

func (t *MemorySearchTool) Description() string {
	return "语义搜索历史知识库。可以搜索漏洞利用方法、CTF 题目解法和代码片段。"
}

func (t *MemorySearchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索查询",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"guide", "answer", "code", "all"},
				"description": "搜索类型（默认: all）",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回结果数量（默认: 5）",
			},
		},
		"required": []string{"query"},
	}
}

func (t *MemorySearchTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 5
	}

	var results []memory.VectorEntry
	var err error

	switch params.Type {
	case "guide":
		results, err = t.memoryMgr.SearchGuides(ctx, params.Query, limit)
	case "answer":
		results, err = t.memoryMgr.SearchAnswers(ctx, params.Query, limit)
	case "code":
		results, err = t.memoryMgr.SearchCode(ctx, params.Query, limit)
	default:
		// 搜索所有类型
		results, err = t.memoryMgr.SearchGuides(ctx, params.Query, limit)
		if err == nil {
			answers, _ := t.memoryMgr.SearchAnswers(ctx, params.Query, limit)
			results = append(results, answers...)
			codes, _ := t.memoryMgr.SearchCode(ctx, params.Query, limit)
			results = append(results, codes...)
		}
	}

	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "没有找到相关的历史知识。", nil
	}

	// 格式化输出
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条相关知识：\n\n", len(results)))

	for i, entry := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, entry.Metadata.Title))
		sb.WriteString(fmt.Sprintf("- 类型: %s\n", entry.Metadata.Type))
		if len(entry.Metadata.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- 标签: %s\n", strings.Join(entry.Metadata.Tags, ", ")))
		}
		sb.WriteString("\n")

		// 截断内容
		content := entry.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(content + "\n\n")
	}

	return sb.String(), nil
}

// MemoryStoreTool 存储知识到记忆
type MemoryStoreTool struct {
	memoryMgr *memory.MemoryManager
}

func NewMemoryStoreTool(memoryMgr *memory.MemoryManager) *MemoryStoreTool {
	return &MemoryStoreTool{memoryMgr: memoryMgr}
}

func (t *MemoryStoreTool) Name() string {
	return "memory_store"
}

func (t *MemoryStoreTool) Description() string {
	return "存储知识到记忆库。用于保存漏洞利用方法、解题思路或代码片段。"
}

func (t *MemoryStoreTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"guide", "answer", "code"},
				"description": "知识类型",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "知识标题",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "知识内容",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "标签列表",
			},
		},
		"required": []string{"type", "title", "content"},
	}
}

func (t *MemoryStoreTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Type    string   `json:"type"`
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Type == "" || params.Title == "" || params.Content == "" {
		return "", fmt.Errorf("type, title, and content are required")
	}

	var err error
	switch params.Type {
	case "guide":
		err = t.memoryMgr.StoreGuide(ctx, params.Title, params.Content, params.Tags)
	case "answer":
		err = t.memoryMgr.StoreAnswer(ctx, params.Title, params.Content, 0, params.Tags)
	case "code":
		err = t.memoryMgr.StoreCode(ctx, params.Title, params.Content, params.Tags)
	default:
		return "", fmt.Errorf("unknown type: %s", params.Type)
	}

	if err != nil {
		return "", fmt.Errorf("store failed: %w", err)
	}

	return fmt.Sprintf("成功存储 %s 类型的知识: %s", params.Type, params.Title), nil
}
