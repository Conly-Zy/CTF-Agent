package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/replay"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

// ReportFormat 报告格式
type ReportFormat string

const (
	FormatMarkdown ReportFormat = "markdown"
	FormatHTML     ReportFormat = "html"
	FormatJSON     ReportFormat = "json"
)

// Report 解题报告
type Report struct {
	Title       string           `json:"title"`
	SessionID   int64            `json:"session_id"`
	Challenge   ChallengeInfo    `json:"challenge"`
	Summary     string           `json:"summary"`
	Timeline    []TimelineEntry  `json:"timeline"`
	Subtasks    []SubtaskSummary `json:"subtasks,omitempty"`
	ToolsUsed   []ToolUsage      `json:"tools_used"`
	Flag        string           `json:"flag"`
	Duration    time.Duration    `json:"duration"`
	Iterations  int              `json:"iterations"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// ChallengeInfo 题目信息
type ChallengeInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Target      string `json:"target"`
}

// TimelineEntry 时间线条目
type TimelineEntry struct {
	Time   time.Time `json:"time"`
	Agent  string    `json:"agent,omitempty"`
	Action string    `json:"action"`
	Detail string    `json:"detail,omitempty"`
}

// ToolUsage 工具使用统计
type ToolUsage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SubtaskSummary is a compact report-facing view of a persisted agent subtask.
type SubtaskSummary struct {
	ID          int64     `json:"id"`
	Agent       string    `json:"agent"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// ReportGenerator 报告生成器
type ReportGenerator struct{}

// NewReportGenerator 创建报告生成器
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{}
}

// GenerateFromSession 从会话生成报告
func (g *ReportGenerator) GenerateFromSession(session *store.Session) *Report {
	report := &Report{
		Title:     fmt.Sprintf("CTF 解题报告 #%d", session.ID),
		SessionID: session.ID,
		Challenge: ChallengeInfo{
			Type:        session.ChallengeType,
			Description: session.Description,
			Target:      session.Target,
		},
		Flag:        session.Flag,
		Iterations:  session.Iterations,
		GeneratedAt: time.Now(),
	}
	if session.DurationMs > 0 {
		report.Duration = time.Duration(session.DurationMs) * time.Millisecond
	}
	report.Summary = g.generateSummary(report)
	return report
}

// GenerateFromReplay 从回放数据生成报告
func (g *ReportGenerator) GenerateFromReplay(session *store.Session, rp *replay.SessionReplay) *Report {
	report := g.GenerateFromSession(session)
	report.Duration = rp.Duration()

	// 统计工具使用
	toolCounts := make(map[string]int)
	for _, call := range rp.ToolCalls() {
		toolCounts[call.Tool]++
	}
	for tool, count := range toolCounts {
		report.ToolsUsed = append(report.ToolsUsed, ToolUsage{Name: tool, Count: count})
	}
	sortToolUsage(report.ToolsUsed)

	// 构建时间线
	for _, event := range rp.Events {
		entry := TimelineEntry{Time: event.Timestamp}
		switch event.Type {
		case "agent_start":
			if data, ok := event.Data.(map[string]interface{}); ok {
				entry.Agent = data["agent_name"].(string)
				entry.Action = "Agent 启动"
			}
		case "agent_complete":
			if data, ok := event.Data.(map[string]interface{}); ok {
				entry.Agent = data["agent_name"].(string)
				entry.Action = "Agent 完成"
			}
		case "tool_start":
			if data, ok := event.Data.(map[string]interface{}); ok {
				entry.Action = "工具调用"
				entry.Detail = data["tool"].(string)
			}
		case "tool_result":
			if data, ok := event.Data.(map[string]interface{}); ok {
				entry.Action = "工具返回"
				entry.Detail = data["tool"].(string)
			}
		case "flag":
			if data, ok := event.Data.(map[string]interface{}); ok {
				entry.Action = "Flag 发现"
				entry.Detail = data["flag"].(string)
			}
		case "thinking":
			entry.Action = "思考"
			if data, ok := event.Data.(map[string]interface{}); ok {
				content := data["content"].(string)
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				entry.Detail = content
			}
		default:
			continue
		}
		report.Timeline = append(report.Timeline, entry)
	}
	sortTimeline(report.Timeline)

	// 生成摘要
	report.Summary = g.generateSummary(report)

	return report
}

// GenerateFromEvidence builds a report from the persistent execution evidence
// captured in subtasks/tool_calls. This is the preferred path after the
// PentAGI-inspired evidence model is available because it does not depend on
// separate replay files.
func (g *ReportGenerator) GenerateFromEvidence(session *store.Session, subtasks []store.Subtask, toolCalls []store.ToolCall) *Report {
	report := g.GenerateFromSession(session)

	toolCounts := make(map[string]int)
	for _, call := range toolCalls {
		toolCounts[call.ToolName]++
		status := "工具调用"
		if call.Status == "failed" {
			status = "工具失败"
		} else if call.Status == "finished" {
			status = "工具完成"
		}
		detail := call.ToolName
		if call.Error != "" {
			detail += ": " + call.Error
		}
		report.Timeline = append(report.Timeline, TimelineEntry{
			Time:   call.StartedAt,
			Agent:  call.AgentName,
			Action: status,
			Detail: detail,
		})
	}
	for tool, count := range toolCounts {
		report.ToolsUsed = append(report.ToolsUsed, ToolUsage{Name: tool, Count: count})
	}
	sortToolUsage(report.ToolsUsed)

	for _, st := range subtasks {
		completedAt := time.Time{}
		if st.CompletedAt != nil {
			completedAt = *st.CompletedAt
		}
		report.Subtasks = append(report.Subtasks, SubtaskSummary{
			ID:          st.ID,
			Agent:       st.AgentName,
			Type:        st.AgentType,
			Status:      st.Status,
			Title:       st.Title,
			Description: st.Description,
			Result:      st.Result,
			Error:       st.Error,
			StartedAt:   st.CreatedAt,
			CompletedAt: completedAt,
		})

		action := "子任务启动"
		if st.Status == "success" {
			action = "子任务成功"
		} else if st.Status == "covered" {
			action = "计划已覆盖"
		} else if st.Status == "needs_review" {
			action = "计划待复盘"
		} else if st.Status == "skipped" {
			action = "计划跳过"
		} else if st.Status == "failed" {
			action = "子任务失败"
		} else if st.Status == "finished" {
			action = "子任务完成"
		}
		detail := st.Title
		if detail == "" {
			detail = st.Description
		}
		if st.Error != "" {
			detail += ": " + st.Error
		}
		report.Timeline = append(report.Timeline, TimelineEntry{
			Time:   st.CreatedAt,
			Agent:  st.AgentName,
			Action: action,
			Detail: detail,
		})
	}

	sortTimeline(report.Timeline)
	report.Summary = g.generateSummary(report)
	return report
}

func sortToolUsage(items []ToolUsage) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
}

func sortTimeline(items []TimelineEntry) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Time.Before(items[j].Time)
	})
}

func (g *ReportGenerator) generateSummary(r *Report) string {
	var sb strings.Builder

	if r.Flag != "" {
		sb.WriteString(fmt.Sprintf("成功解题，找到 Flag: `%s`。", r.Flag))
	} else {
		sb.WriteString("未能找到 Flag。")
	}

	sb.WriteString(fmt.Sprintf(" 题目类型为 %s，共迭代 %d 次", r.Challenge.Type, r.Iterations))

	if r.Duration > 0 {
		sb.WriteString(fmt.Sprintf("，耗时 %s", r.Duration.Round(time.Second)))
	}

	sb.WriteString("。")

	if len(r.ToolsUsed) > 0 {
		sb.WriteString(" 使用的工具包括：")
		for i, tool := range r.ToolsUsed {
			if i > 0 {
				sb.WriteString("、")
			}
			sb.WriteString(fmt.Sprintf("%s (%d次)", tool.Name, tool.Count))
		}
		sb.WriteString("。")
	}

	return sb.String()
}

// RenderMarkdown 渲染为 Markdown
func (g *ReportGenerator) RenderMarkdown(r *Report) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", r.Title))
	sb.WriteString(fmt.Sprintf("生成时间: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05")))

	// 摘要
	sb.WriteString("## 摘要\n\n")
	sb.WriteString(r.Summary + "\n\n")

	// 题目信息
	sb.WriteString("## 题目信息\n\n")
	sb.WriteString(fmt.Sprintf("- **类型**: %s\n", r.Challenge.Type))
	if r.Challenge.Description != "" {
		sb.WriteString(fmt.Sprintf("- **描述**: %s\n", r.Challenge.Description))
	}
	if r.Challenge.Target != "" {
		sb.WriteString(fmt.Sprintf("- **目标**: %s\n", r.Challenge.Target))
	}
	sb.WriteString("\n")

	// 结果
	sb.WriteString("## 解题结果\n\n")
	sb.WriteString(fmt.Sprintf("- **状态**: %s\n", map[bool]string{true: "成功", false: "失败"}[r.Flag != ""]))
	if r.Flag != "" {
		sb.WriteString(fmt.Sprintf("- **Flag**: `%s`\n", r.Flag))
	}
	sb.WriteString(fmt.Sprintf("- **迭代次数**: %d\n", r.Iterations))
	if r.Duration > 0 {
		sb.WriteString(fmt.Sprintf("- **耗时**: %s\n", r.Duration.Round(time.Second)))
	}
	sb.WriteString("\n")

	// 工具使用
	if len(r.ToolsUsed) > 0 {
		sb.WriteString("## 工具使用统计\n\n")
		sb.WriteString("| 工具 | 调用次数 |\n")
		sb.WriteString("|------|----------|\n")
		for _, tool := range r.ToolsUsed {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", tool.Name, tool.Count))
		}
		sb.WriteString("\n")
	}

	// 子任务
	if len(r.Subtasks) > 0 {
		sb.WriteString("## Agent 子任务\n\n")
		sb.WriteString("| ID | Agent | 类型 | 状态 | 标题 |\n")
		sb.WriteString("|----|-------|------|------|------|\n")
		for _, st := range r.Subtasks {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
				st.ID, st.Agent, st.Type, st.Status, oneLine(st.Title, 80)))
		}
		sb.WriteString("\n")
	}

	// 时间线
	if len(r.Timeline) > 0 {
		sb.WriteString("## 执行时间线\n\n")
		for _, entry := range r.Timeline {
			timeStr := entry.Time.Format("15:04:05")
			line := fmt.Sprintf("- **%s** [%s]", timeStr, entry.Action)
			if entry.Agent != "" {
				line += fmt.Sprintf(" `%s`", entry.Agent)
			}
			if entry.Detail != "" {
				detail := entry.Detail
				if len(detail) > 80 {
					detail = detail[:80] + "..."
				}
				line += fmt.Sprintf(": %s", detail)
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func oneLine(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
