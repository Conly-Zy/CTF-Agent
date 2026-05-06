package agent

import (
	"log/slog"
	"os"
	"testing"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
)

func TestSummarizer_ShouldSummarize(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	summarizer := NewSummarizer(nil, logger, 5, 2)

	tests := []struct {
		name     string
		messages []llm.Message
		expected bool
	}{
		{
			name:     "below threshold",
			messages: make([]llm.Message, 3),
			expected: false,
		},
		{
			name:     "at threshold",
			messages: make([]llm.Message, 5),
			expected: false,
		},
		{
			name:     "above threshold",
			messages: make([]llm.Message, 6),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizer.ShouldSummarize(tt.messages)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSummarizer_ExtractContent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	summarizer := NewSummarizer(nil, logger, 5, 2)

	tests := []struct {
		name     string
		msg      llm.Message
		expected string
	}{
		{
			name:     "text message",
			msg:      llm.NewTextMessage("user", "hello"),
			expected: "hello",
		},
		{
			name: "tool use message",
			msg: llm.Message{
				Role: "assistant",
				Content: []llm.ContentBlock{
					{Type: "text", Text: "using tool"},
					{Type: "tool_use", Name: "shell_exec"},
				},
			},
			expected: "using tool[工具调用: shell_exec]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizer.extractContent(tt.msg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
