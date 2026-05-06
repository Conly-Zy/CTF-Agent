package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// BarrierType 屏障类型
type BarrierType string

const (
	BarrierDone BarrierType = "done" // 任务完成
	BarrierAsk  BarrierType = "ask"  // 请求协助
)

// BarrierPayload 屏障工具参数
type BarrierPayload struct {
	Type     BarrierType `json:"type"`
	Flag     string      `json:"flag,omitempty"`     // done 时提供 flag
	Summary  string      `json:"summary"`            // 结果摘要
	Question string      `json:"question,omitempty"` // ask 时的问题
	Context  string      `json:"context,omitempty"`  // 上下文信息
}

// DoneBarrierTool 专用的完成屏障
type DoneBarrierTool struct {
	resultCh chan<- BarrierPayload
}

func NewDoneBarrierTool(resultCh chan<- BarrierPayload) *DoneBarrierTool {
	return &DoneBarrierTool{
		resultCh: resultCh,
	}
}

func (t *DoneBarrierTool) Name() string {
	return "barrier_done"
}

func (t *DoneBarrierTool) Description() string {
	return "标记任务完成并返回找到的 flag。当成功解题时使用此工具。"
}

func (t *DoneBarrierTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"flag": map[string]any{
				"type":        "string",
				"description": "找到的 flag",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "解题过程摘要",
			},
		},
		"required": []string{"flag", "summary"},
	}
}

func (t *DoneBarrierTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var payload struct {
		Flag    string `json:"flag"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return "", fmt.Errorf("parse done barrier input: %w", err)
	}

	if payload.Flag == "" {
		return "", fmt.Errorf("flag is required for done barrier")
	}

	barrier := BarrierPayload{
		Type:    BarrierDone,
		Flag:    payload.Flag,
		Summary: payload.Summary,
	}

	select {
	case t.resultCh <- barrier:
		return fmt.Sprintf("Task completed successfully. Flag: %s", payload.Flag), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// AskBarrierTool 专用的协助屏障
type AskBarrierTool struct {
	resultCh  chan<- BarrierPayload
	answersCh <-chan string
}

func NewAskBarrierTool(resultCh chan<- BarrierPayload, answersCh <-chan string) *AskBarrierTool {
	return &AskBarrierTool{
		resultCh:  resultCh,
		answersCh: answersCh,
	}
}

func (t *AskBarrierTool) Name() string {
	return "barrier_ask"
}

func (t *AskBarrierTool) Description() string {
	return "向 Primary Agent 或其他专业 Agent 请求协助。用于需要跨领域知识的情况。"
}

func (t *AskBarrierTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "需要协助的问题",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "相关上下文信息",
			},
		},
		"required": []string{"question"},
	}
}

func (t *AskBarrierTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var payload struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return "", fmt.Errorf("parse ask barrier input: %w", err)
	}

	if payload.Question == "" {
		return "", fmt.Errorf("question is required for ask barrier")
	}

	barrier := BarrierPayload{
		Type:     BarrierAsk,
		Question: payload.Question,
		Context:  payload.Context,
	}

	// 发送问题
	select {
	case t.resultCh <- barrier:
		// 等待回答
		select {
		case answer := <-t.answersCh:
			return answer, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
