package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider OpenAI 兼容提供者
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, model, baseURL string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (p *OpenAIProvider) Name() string { return "openai" }

type openaiRequest struct {
	Model       string           `json:"model"`
	Messages    []openaiMessage  `json:"messages"`
	Tools       []openaiTool     `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiFunctionDef  `json:"function"`
}

type openaiFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *OpenAIProvider) CreateMessage(ctx context.Context, params MessageParams) (*MessageResponse, error) {
	reqBody := openaiRequest{
		Model:     p.model,
		Messages:  p.convertMessages(params),
		MaxTokens: params.MaxTokens,
	}

	for _, tool := range params.Tools {
		reqBody.Tools = append(reqBody.Tools, openaiTool{
			Type: "function",
			Function: openaiFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	return p.convertResponse(&openaiResp), nil
}

func (p *OpenAIProvider) CreateMessageStream(ctx context.Context, params MessageParams, writer io.Writer) (*MessageResponse, error) {
	reqBody := openaiRequest{
		Model:     p.model,
		Messages:  p.convertMessages(params),
		MaxTokens: params.MaxTokens,
		Stream:    true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	// Read SSE stream
	decoder := json.NewDecoder(resp.Body)
	var fullContent string

	for {
		var line struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := decoder.Decode(&line); err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		for _, choice := range line.Choices {
			if choice.Delta.Content != "" {
				writer.Write([]byte(choice.Delta.Content))
				fullContent += choice.Delta.Content
			}
		}
	}

	return &MessageResponse{
		Content:    []ContentBlock{{Type: "text", Text: fullContent}},
		StopReason: "end_turn",
	}, nil
}

func (p *OpenAIProvider) convertMessages(params MessageParams) []openaiMessage {
	var msgs []openaiMessage

	if params.SystemPrompt != "" {
		msgs = append(msgs, openaiMessage{Role: "system", Content: params.SystemPrompt})
	}

	for _, msg := range params.Messages {
		switch content := msg.Content.(type) {
		case string:
			msgs = append(msgs, openaiMessage{Role: msg.Role, Content: content})
		case []ContentBlock:
			if msg.Role == "assistant" {
				m := openaiMessage{Role: "assistant"}
				for _, b := range content {
					switch b.Type {
					case "text":
						m.Content = b.Text
					case "tool_use":
						m.ToolCalls = append(m.ToolCalls, openaiToolCall{
							ID:   b.ID,
							Type: "function",
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name:      b.Name,
								Arguments: string(b.Input),
							},
						})
					}
				}
				msgs = append(msgs, m)
			} else {
				for _, b := range content {
					switch b.Type {
					case "tool_result":
						msgs = append(msgs, openaiMessage{
							Role:       "tool",
							Content:    b.Content,
							ToolCallID: b.ToolUseID,
						})
					case "text":
						msgs = append(msgs, openaiMessage{Role: "user", Content: b.Text})
					}
				}
			}
		}
	}

	return msgs
}

func (p *OpenAIProvider) convertResponse(resp *openaiResponse) *MessageResponse {
	if len(resp.Choices) == 0 {
		return &MessageResponse{}
	}

	choice := resp.Choices[0]
	msgResp := &MessageResponse{
		StopReason: choice.FinishReason,
	}

	if choice.Message.Content != "" {
		msgResp.Content = append(msgResp.Content, ContentBlock{
			Type: "text",
			Text: choice.Message.Content,
		})
	}

	for _, tc := range choice.Message.ToolCalls {
		msgResp.Content = append(msgResp.Content, ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return msgResp
}
