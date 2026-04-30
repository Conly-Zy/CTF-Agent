package knowledge

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

type Extractor struct {
	store *store.SQLiteStore
}

func NewExtractor(store *store.SQLiteStore) *Extractor {
	return &Extractor{store: store}
}

type ExtractionResult struct {
	Knowledge *store.Knowledge
	Tags      []string
}

func (e *Extractor) ExtractFromSession(sess *store.Session, messages []store.ConversationMessage) (*ExtractionResult, error) {
	title := e.generateTitle(sess)
	content := e.generateMarkdown(sess, messages)
	knowledgeType := e.determineType(sess)
	tags := e.extractTags(sess, messages)

	k := &store.Knowledge{
		SessionID: sess.ID,
		Title:     title,
		Content:   content,
		Type:      knowledgeType,
		CreatedAt: time.Now(),
	}

	if err := e.store.CreateKnowledge(k); err != nil {
		return nil, fmt.Errorf("create knowledge: %w", err)
	}

	for _, tagName := range tags {
		tag, err := e.store.CreateTag(tagName)
		if err != nil {
			continue
		}
		e.store.AddTagToKnowledge(k.ID, tag.ID)
	}

	return &ExtractionResult{
		Knowledge: k,
		Tags:      tags,
	}, nil
}

func (e *Extractor) generateTitle(sess *store.Session) string {
	if sess.Description != "" {
		desc := sess.Description
		if len(desc) > 50 {
			desc = desc[:50] + "..."
		}
		return fmt.Sprintf("[%s] %s", strings.ToUpper(sess.ChallengeType), desc)
	}
	return fmt.Sprintf("[%s] Session #%d", strings.ToUpper(sess.ChallengeType), sess.ID)
}

func (e *Extractor) determineType(sess *store.Session) string {
	switch sess.ChallengeType {
	case "web":
		return "vulnerability"
	case "pwn":
		return "exploit"
	case "crypto":
		return "technique"
	case "reverse":
		return "analysis"
	default:
		return "technique"
	}
}

func (e *Extractor) extractTags(sess *store.Session, messages []store.ConversationMessage) []string {
	tags := []string{sess.ChallengeType}

	keywordPatterns := map[string][]string{
		"web":     {"sql_injection", "xss", "csrf", "ssti", "lfi", "rfi", "xxe", "ssrf", "deserialization"},
		"pwn":     {"buffer_overflow", "format_string", "heap", "rop", "shellcode", "ret2libc"},
		"crypto":  {"rsa", "aes", "xor", "hash", "ecb", "cbc", "padding_oracle", "lattice"},
		"reverse": {"disassembly", "decompile", "anti_debug", "obfuscation", "unpacking"},
	}

	content := e.getFullContent(messages)
	contentLower := strings.ToLower(content)

	if patterns, ok := keywordPatterns[sess.ChallengeType]; ok {
		for _, pattern := range patterns {
			if strings.Contains(contentLower, strings.ReplaceAll(pattern, "_", " ")) ||
				strings.Contains(contentLower, pattern) {
				tags = append(tags, pattern)
			}
		}
	}

	toolPatterns := regexp.MustCompile(`(?:using|tool|command|script).*?(sqlmap|nikto|nmap|gdb|pwntools|angr|z3|john|hashcat)`)
	matches := toolPatterns.FindAllStringSubmatch(contentLower, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tags = append(tags, match[1])
		}
	}

	return e.uniqueTags(tags)
}

func (e *Extractor) getFullContent(messages []store.ConversationMessage) string {
	var parts []string
	for _, msg := range messages {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func (e *Extractor) uniqueTags(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

func (e *Extractor) generateMarkdown(sess *store.Session, messages []store.ConversationMessage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", e.generateTitle(sess)))

	sb.WriteString("## 基本信息\n\n")
	sb.WriteString(fmt.Sprintf("- **类型**: %s\n", strings.ToUpper(sess.ChallengeType)))
	sb.WriteString(fmt.Sprintf("- **状态**: %s\n", sess.Status))
	if sess.Target != "" {
		sb.WriteString(fmt.Sprintf("- **目标**: %s\n", sess.Target))
	}
	sb.WriteString(fmt.Sprintf("- **迭代次数**: %d\n", sess.Iterations))
	sb.WriteString(fmt.Sprintf("- **耗时**: %v\n", time.Duration(sess.DurationMs)*time.Millisecond))
	sb.WriteString(fmt.Sprintf("- **创建时间**: %s\n\n", sess.CreatedAt.Format("2006-01-02 15:04:05")))

	if sess.Description != "" {
		sb.WriteString("## 题目描述\n\n")
		sb.WriteString(sess.Description + "\n\n")
	}

	sb.WriteString("## 解题过程\n\n")

	step := 1
	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			if msg.Content != "" {
				sb.WriteString(fmt.Sprintf("### 步骤 %d: 分析\n\n", step))
				sb.WriteString(msg.Content + "\n\n")
				step++
			}
		case "user":
			if msg.ToolName != "" {
				sb.WriteString(fmt.Sprintf("### 工具调用: %s\n\n", msg.ToolName))
				sb.WriteString(fmt.Sprintf("**输入**:\n```json\n%s\n```\n\n", msg.ToolInput))
				if msg.Content != "" {
					sb.WriteString(fmt.Sprintf("**输出**:\n```\n%s\n```\n\n", msg.Content))
				}
			}
		}
	}

	if sess.Flag != "" {
		sb.WriteString("## Flag\n\n")
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", sess.Flag))
	}

	if sess.Error != "" {
		sb.WriteString("## 错误信息\n\n")
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", sess.Error))
	}

	return sb.String()
}
