package llm

import "encoding/json"

type MessageParams struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
}

type Message struct {
	Role    string
	Content string
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

type ContentBlock struct {
	Type  string
	Text  string
	ID    string
	Name  string
	Input json.RawMessage
}

func (r *MessageResponse) GetText() string {
	for _, block := range r.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
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
