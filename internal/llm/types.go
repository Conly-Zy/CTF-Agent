package llm

import "encoding/json"

type MessageParams struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
}

// Message supports structured content blocks for proper tool_use/tool_result flow.
type Message struct {
	Role    string
	Content any // string, []ContentBlock, or nil
}

// ContentBlock represents a block in a message (text, tool_use, or tool_result).
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// NewTextMessage creates a simple text message.
func NewTextMessage(role, text string) Message {
	return Message{Role: role, Content: text}
}

// NewToolUseMessage creates an assistant message with tool_use blocks.
func NewToolUseMessage(text string, toolUses []ContentBlock) Message {
	var blocks []ContentBlock
	if text != "" {
		blocks = append(blocks, ContentBlock{Type: "text", Text: text})
	}
	blocks = append(blocks, toolUses...)
	return Message{Role: "assistant", Content: blocks}
}

// NewToolResultMessage creates a user message with tool_result blocks.
func NewToolResultMessage(results []ContentBlock) Message {
	return Message{Role: "user", Content: results}
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type MessageResponse struct {
	ID         string
	Content    []ContentBlock
	StopReason string
}

func (r *MessageResponse) GetText() string {
	var text string
	for _, block := range r.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}

func (r *MessageResponse) GetToolUse() []ContentBlock {
	var toolUses []ContentBlock
	for _, block := range r.Content {
		if block.Type == "tool_use" {
			toolUses = append(toolUses, block)
		}
	}
	return toolUses
}
