package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
)

// Summarizer 负责压缩长对话历史
type Summarizer struct {
	llmClient   *llm.Client
	logger      *slog.Logger
	maxMessages int // 触发压缩的消息数量阈值
	keepRecent  int // 保留最近的消息数量
}

func NewSummarizer(llmClient *llm.Client, logger *slog.Logger, maxMessages, keepRecent int) *Summarizer {
	return &Summarizer{
		llmClient:   llmClient,
		logger:      logger,
		maxMessages: maxMessages,
		keepRecent:  keepRecent,
	}
}

// ShouldSummarize 检查是否需要压缩
func (s *Summarizer) ShouldSummarize(messages []llm.Message) bool {
	return len(messages) > s.maxMessages
}

// Summarize 压缩消息历史
func (s *Summarizer) Summarize(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
	if len(messages) <= s.keepRecent {
		return messages, nil
	}

	s.logger.Info("summarizing messages",
		"total", len(messages),
		"keep_recent", s.keepRecent)

	// 分割消息：历史部分和最近部分
	historyEnd := len(messages) - s.keepRecent
	historyMessages := messages[:historyEnd]
	recentMessages := messages[historyEnd:]

	// 构建摘要提示
	summaryPrompt := s.buildSummaryPrompt(historyMessages)

	// 调用 LLM 生成摘要
	resp, err := s.llmClient.CreateMessage(ctx, llm.MessageParams{
		SystemPrompt: "你是一个对话摘要助手。请将以下对话历史压缩为简洁的摘要，保留关键信息和技术细节。",
		Messages:     []llm.Message{llm.NewTextMessage("user", summaryPrompt)},
		MaxTokens:    1024,
	})
	if err != nil {
		return nil, fmt.Errorf("summarize failed: %w", err)
	}

	summaryText := resp.GetText()
	if summaryText == "" {
		return messages, nil
	}

	// 构建压缩后的消息列表
	summarizedMessages := []llm.Message{
		llm.NewTextMessage("user", fmt.Sprintf("**summarized content:**\n%s", summaryText)),
	}
	summarizedMessages = append(summarizedMessages, recentMessages...)

	s.logger.Info("summarization completed",
		"original", len(messages),
		"summarized", len(summarizedMessages))

	return summarizedMessages, nil
}

// buildSummaryPrompt 构建摘要提示
func (s *Summarizer) buildSummaryPrompt(messages []llm.Message) string {
	prompt := "请将以下对话历史压缩为简洁的摘要：\n\n"

	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "AI"
		}

		// 提取文本内容
		content := s.extractContent(msg)

		if content != "" {
			prompt += fmt.Sprintf("%s: %s\n", role, content)
		}
	}

	prompt += "\n请提供简洁的摘要，保留：\n"
	prompt += "1. 已经尝试的方法和结果\n"
	prompt += "2. 发现的关键信息\n"
	prompt += "3. 当前的进展状态\n"
	prompt += "4. 下一步应该尝试的方向"

	return prompt
}

// extractContent 从消息中提取内容
func (s *Summarizer) extractContent(msg llm.Message) string {
	switch content := msg.Content.(type) {
	case string:
		return content
	case []llm.ContentBlock:
		result := ""
		for _, block := range content {
			if block.Type == "text" {
				result += block.Text
			} else if block.Type == "tool_use" {
				result += fmt.Sprintf("[工具调用: %s]", block.Name)
			} else if block.Type == "tool_result" {
				result += fmt.Sprintf("[工具结果: %s]", block.Content)
			}
		}
		return result
	default:
		return ""
	}
}

// CompressChain 压缩消息链（保留最近 N 字节）
func (s *Summarizer) CompressChain(messages []llm.Message, maxBytes int) []llm.Message {
	totalBytes := 0
	for _, msg := range messages {
		totalBytes += s.getMessageBytes(msg)
	}

	if totalBytes <= maxBytes {
		return messages
	}

	s.logger.Info("compressing chain",
		"total_bytes", totalBytes,
		"max_bytes", maxBytes)

	// 从后往前保留消息，直到达到字节限制
	compressedBytes := 0
	keepIndex := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		msgBytes := s.getMessageBytes(messages[i])

		if compressedBytes+msgBytes > maxBytes {
			break
		}
		compressedBytes += msgBytes
		keepIndex = i
	}

	// 如果需要压缩，在前面添加压缩标记
	if keepIndex > 0 {
		compressed := []llm.Message{
			llm.NewTextMessage("user", "**注意：之前的消息已被压缩，以下是最新的对话内容**"),
		}
		compressed = append(compressed, messages[keepIndex:]...)
		return compressed
	}

	return messages
}

// getMessageBytes 获取消息的字节数
func (s *Summarizer) getMessageBytes(msg llm.Message) int {
	switch content := msg.Content.(type) {
	case string:
		return len(content)
	case []llm.ContentBlock:
		bytes := 0
		for _, block := range content {
			bytes += len(block.Text) + len(block.Content)
		}
		return bytes
	default:
		return 0
	}
}
