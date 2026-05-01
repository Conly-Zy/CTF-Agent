package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Client struct {
	client *anthropic.Client
	model  string
}

func NewClient(apiKey string, model string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &Client{
		client: &client,
		model:  model,
	}, nil
}

func (c *Client) CreateMessage(ctx context.Context, params MessageParams) (*MessageResponse, error) {
	messages := convertMessages(params.Messages)

	tools := make([]anthropic.ToolUnionParam, len(params.Tools))
	for i, tool := range params.Tools {
		schema := convertSchema(tool.InputSchema)
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: schema,
			},
		}
	}

	req := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: int64(params.MaxTokens),
		Messages:  messages,
	}

	if params.SystemPrompt != "" {
		req.System = []anthropic.TextBlockParam{
			{Text: params.SystemPrompt},
		}
	}

	if len(tools) > 0 {
		req.Tools = tools
	}

	resp, err := c.client.Messages.New(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	return convertResponse(resp), nil
}

func (c *Client) CreateMessageStream(ctx context.Context, params MessageParams, writer io.Writer) (*MessageResponse, error) {
	messages := convertMessages(params.Messages)

	tools := make([]anthropic.ToolUnionParam, len(params.Tools))
	for i, tool := range params.Tools {
		schema := convertSchema(tool.InputSchema)
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: schema,
			},
		}
	}

	req := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: int64(params.MaxTokens),
		Messages:  messages,
	}

	if params.SystemPrompt != "" {
		req.System = []anthropic.TextBlockParam{
			{Text: params.SystemPrompt},
		}
	}

	if len(tools) > 0 {
		req.Tools = tools
	}

	stream := c.client.Messages.NewStreaming(ctx, req)

	var finalMsg anthropic.Message
	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" {
			if event.Delta.Text != "" {
				writer.Write([]byte(event.Delta.Text))
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	finalMsg = stream.Current().Message

	return convertResponse(&finalMsg), nil
}

// convertMessages converts our Message type to Anthropic SDK message params,
// properly handling text, tool_use, and tool_result content blocks.
func convertMessages(msgs []Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, msg := range msgs {
		switch content := msg.Content.(type) {
		case string:
			// Simple text message
			if msg.Role == "user" {
				result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))
			} else {
				result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(content)))
			}

		case []ContentBlock:
			if msg.Role == "assistant" {
				var blocks []anthropic.ContentBlockParamUnion
				for _, b := range content {
					switch b.Type {
					case "text":
						blocks = append(blocks, anthropic.NewTextBlock(b.Text))
					case "tool_use":
						var inputMap map[string]any
						json.Unmarshal(b.Input, &inputMap)
						blocks = append(blocks, anthropic.NewToolUseBlock(b.ID, inputMap, b.Name))
					}
				}
				result = append(result, anthropic.NewAssistantMessage(blocks...))

			} else if msg.Role == "user" {
				var blocks []anthropic.ContentBlockParamUnion
				for _, b := range content {
					switch b.Type {
					case "tool_result":
						blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
					case "text":
						blocks = append(blocks, anthropic.NewTextBlock(b.Text))
					}
				}
				result = append(result, anthropic.NewUserMessage(blocks...))
			}
		}
	}

	return result
}

func convertSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	s := anthropic.ToolInputSchemaParam{
		Type: "object",
	}

	if props, ok := schema["properties"]; ok {
		s.Properties = props
	}

	if required, ok := schema["required"]; ok {
		if reqSlice, ok := required.([]string); ok {
			s.Required = reqSlice
		}
	}

	return s
}

func convertResponse(msg *anthropic.Message) *MessageResponse {
	resp := &MessageResponse{
		ID:         msg.ID,
		StopReason: string(msg.StopReason),
	}

	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Content = append(resp.Content, ContentBlock{
				Type: "text",
				Text: b.Text,
			})
		case anthropic.ToolUseBlock:
			input, _ := json.Marshal(b.Input)
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "tool_use",
				ID:    b.ID,
				Name:  b.Name,
				Input: input,
			})
		}
	}

	return resp
}
