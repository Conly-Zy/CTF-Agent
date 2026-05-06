package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// SessionControls provides external control signals for a solving session.
type SessionControls struct {
	PauseCh   <-chan struct{} // receive when pause is requested
	ResumeCh  <-chan struct{} // receive when resume is requested
	InjectCh  <-chan string   // receive injected user messages
}

type Orchestrator struct {
	llmClient      *llm.Client
	toolRegistry   *tools.Registry
	logger         *slog.Logger
	store          Store
	knowledgeStore KnowledgeStore
	maxIter        int
	timeout        time.Duration
	flagPatterns   []*regexp.Regexp
}

type SolveRequest struct {
	ChallengeType string
	Description   string
	Target        string
	Files         []string
}

type SolveResult struct {
	Success     bool
	Flag        string
	Iterations  int
	Duration    time.Duration
	CompletedAt time.Time
	Error       error
}

func NewOrchestrator(llmClient *llm.Client, toolRegistry *tools.Registry, logger *slog.Logger, maxIter int, timeout time.Duration) *Orchestrator {
	return &Orchestrator{
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		logger:       logger,
		maxIter:      maxIter,
		timeout:      timeout,
	}
}

func (o *Orchestrator) SetStore(store Store) {
	o.store = store
}

func (o *Orchestrator) SetKnowledgeStore(ks KnowledgeStore) {
	o.knowledgeStore = ks
}

func (o *Orchestrator) SetFlagPatterns(patterns []string) {
	o.flagPatterns = nil
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			o.flagPatterns = append(o.flagPatterns, re)
		}
	}
}

func (o *Orchestrator) Solve(ctx context.Context, req SolveRequest) (*SolveResult, error) {
	return o.SolveWithControls(ctx, req, nil, nil)
}

func (o *Orchestrator) SolveWithCallback(req SolveRequest, log Logger) (*SolveResult, error) {
	return o.SolveWithControls(context.Background(), req, log, nil)
}

func (o *Orchestrator) SolveWithControls(ctx context.Context, req SolveRequest, log Logger, controls *SessionControls) (*SolveResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	if log != nil {
		log.Log("info", "构建系统提示...")
	}

	systemPrompt := o.buildSystemPrompt(req)
	userMsg := llm.NewTextMessage("user", o.buildUserMessage(req))
	messages := []llm.Message{userMsg}

	claudeTools := o.toolRegistry.ToClaudeTools()
	toolDefs := make([]llm.ToolDefinition, len(claudeTools))
	for i, t := range claudeTools {
		toolDefs[i] = llm.ToolDefinition{
			Name:        t["name"].(string),
			Description: t["description"].(string),
			InputSchema: t["input_schema"].(map[string]any),
		}
	}

	if log != nil {
		log.Log("info", "开始解题循环...")
	}

	for i := 0; i < o.maxIter; i++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return &SolveResult{
				Success:     false,
				Iterations:  i,
				Duration:    time.Since(start),
				CompletedAt: time.Now(),
				Error:       fmt.Errorf("会话已取消"),
			}, nil
		}

		// Check pause signal
		if controls != nil {
			select {
			case <-controls.PauseCh:
				if log != nil {
					log.Log("info", "会话已暂停，等待恢复...")
				}
				// Block until resume or context cancel
				select {
				case <-controls.ResumeCh:
					if log != nil {
						log.Log("info", "会话已恢复")
					}
				case <-ctx.Done():
					return &SolveResult{
						Success:     false,
						Iterations:  i,
						Duration:    time.Since(start),
						CompletedAt: time.Now(),
						Error:       fmt.Errorf("会话已取消"),
					}, nil
				}
			default:
			}

			// Check for injected messages (non-blocking drain)
			for {
				select {
				case msg := <-controls.InjectCh:
					messages = append(messages, llm.NewTextMessage("user", msg))
					if log != nil {
						log.Log("info", "收到手动干预消息")
					}
				default:
					goto doneInject
				}
			}
		doneInject:
		}

		o.logger.Info("iteration", "count", i+1)

		if log != nil {
			log.Log("info", fmt.Sprintf("迭代 %d/%d", i+1, o.maxIter))
		}

		resp, err := o.llmClient.CreateMessage(ctx, llm.MessageParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        toolDefs,
			MaxTokens:    4096,
		})
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// Log thinking/text
		text := resp.GetText()
		if text != "" && log != nil {
			log.Thinking(text)
		}

		// Check for flag in text response
		if flag := o.extractFlag(text); flag != "" {
			o.logger.Info("flag found", "flag", flag)
			if log != nil {
				log.Flag(flag)
			}
			return &SolveResult{
				Success:     true,
				Flag:        flag,
				Iterations:  i + 1,
				Duration:    time.Since(start),
				CompletedAt: time.Now(),
			}, nil
		}

		// Get tool uses from response
		toolUses := resp.GetToolUse()

		// No tool calls — check stop reason
		if len(toolUses) == 0 {
			if resp.StopReason == "end_turn" {
				if log != nil {
					log.Log("warning", "Agent 结束回合但未找到 Flag")
				}
				continue
			}
			continue
		}

		// Build assistant message with text + tool_use blocks
		messages = append(messages, llm.NewToolUseMessage(text, toolUses))

		// Execute each tool and collect results
		var toolResults []llm.ContentBlock
		for _, tu := range toolUses {
			if log != nil {
				log.ToolStart(tu.Name)
			}

			tool, ok := o.toolRegistry.Get(tu.Name)
			if !ok {
				errMsg := fmt.Sprintf("工具未找到: %s", tu.Name)
				if log != nil {
					log.Log("error", errMsg)
				}
				toolResults = append(toolResults, llm.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tu.ID,
					Content:   errMsg,
					IsError:   true,
				})
				continue
			}

			output, err := tool.Execute(ctx, tu.Input)
			if err != nil {
				errMsg := fmt.Sprintf("工具执行错误: %v", err)
				if log != nil {
					log.Log("error", errMsg)
				}
				output = errMsg
			}

			if log != nil {
				log.ToolResult(tu.Name, output)
			}

			// Check for flag in tool output
			if flag := o.extractFlag(output); flag != "" {
				o.logger.Info("flag found in tool output", "flag", flag)
				if log != nil {
					log.Flag(flag)
				}
				return &SolveResult{
					Success:     true,
					Flag:        flag,
					Iterations:  i + 1,
					Duration:    time.Since(start),
					CompletedAt: time.Now(),
				}, nil
			}

			toolResults = append(toolResults, llm.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tu.ID,
				Content:   output,
			})
		}

		// Add tool results as a single user message
		messages = append(messages, llm.NewToolResultMessage(toolResults))
	}

	return &SolveResult{
		Success:     false,
		Iterations:  o.maxIter,
		Duration:    time.Since(start),
		CompletedAt: time.Now(),
		Error:       fmt.Errorf("达到最大迭代次数仍未找到 Flag"),
	}, nil
}

var systemPrompts = map[string]string{
	"web": `你是一名资深 Web 安全 CTF 选手。你擅长 SQL 注入、XSS、SSRF、文件上传、反序列化、命令注入、CSRF、认证绕过、路径遍历等 Web 漏洞的发现和利用。

解题方法论：
1. 信息收集：检查目标 URL 的响应头、HTML 源码、JS 文件、robots.txt、sitemap.xml
2. 端点发现：枚举隐藏路径和参数
3. 漏洞识别：根据信息判断可能的漏洞类型
4. 利用验证：构造 PoC 验证漏洞存在
5. Flag 提取：利用漏洞读取 flag 文件或触发 flag 输出

常用工具：sqlmap、nikto、gobuster、curl、Burp Suite
常用命令：curl -v、grep -r、find / -name flag*、cat /etc/passwd`,

	"pwn": `你是一名资深二进制漏洞利用 (Pwn) CTF 选手。你擅长栈溢出、堆利用、格式化字符串、ROP 链构造、GOT 覆写、ret2libc 等技术。

解题方法论：
1. 二进制分析：检查文件类型、架构、保护机制（RELRO/Canary/NX/PIE）
2. 逆向分析：理解程序逻辑，找到漏洞函数
3. 漏洞定位：确定溢出点、偏移量
4. 利用开发：构造 payload，绕过保护
5. Flag 提取：获取 shell 或直接读取 flag

常用工具：pwntools、GDB、ROPgadget、one_gadget、checksec
常用命令：file、checksec、objdump -d、readelf -s、ldd`,

	"crypto": `你是一名资深密码学 CTF 选手。你擅长 RSA、AES、DES、椭圆曲线、哈希碰撞、维吉尼亚密码、异或加密等密码算法的分析和破解。

解题方法论：
1. 密文分析：识别加密算法类型和参数
2. 弱点发现：检查密钥长度、随机数质量、已知漏洞
3. 数学攻击：利用数论性质（小指数、共模、因式分解等）
4. 工具辅助：使用 Python/sage 进行计算
5. Flag 提取：解密得到明文 flag

常用工具：Python、SageMath、RsaCtfTool、hashcat、John the Ripper
常用技巧：Wiener 攻击、Hastad 广播攻击、中国剩余定理、Padding Oracle`,

	"reverse": `你是一名资深逆向工程 CTF 选手。你擅长 x86/x64/ARM 汇编、ELF/PE 文件分析、反调试绕过、混淆代码还原、动态分析等技术。

解题方法论：
1. 静态分析：使用 strings、file、readelf 获取基本信息
2. 反汇编：分析关键函数的汇编逻辑
3. 动态调试：GDB 跟踪关键变量和分支
4. 算法还原：识别加密/校验算法并逆向
5. Flag 提取：编写脚本还原 flag 或 patch 程序

常用工具：GDB、IDA Pro、Ghidra、radare2、ltrace、strace
常用命令：strings、objdump -d、readelf -a、ltrace、strace -f`,
}

func (o *Orchestrator) buildSystemPrompt(req SolveRequest) string {
	prompt, ok := systemPrompts[req.ChallengeType]
	if !ok {
		prompt = "你是一名资深 CTF 选手，擅长各类安全挑战的解决。"
	}

	// Inject historical knowledge
	if o.knowledgeStore != nil {
		entries, err := o.knowledgeStore.SearchKnowledgeByType(req.ChallengeType, 3)
		if err == nil && len(entries) > 0 {
			prompt += "\n\n## 历史解题经验\n以下是过去成功解题的经验总结，可以参考：\n"
			for i, e := range entries {
				prompt += fmt.Sprintf("\n### 经验 %d: %s\n", i+1, e.Title)
				// Truncate content to avoid token overflow
				content := e.Content
				if len(content) > 800 {
					content = content[:800] + "..."
				}
				prompt += content + "\n"
			}
		}
	}

	// Build tool list from registry
	toolNames := o.toolRegistry.Names()
	toolDesc := "\n## 可用工具\n"
	for _, name := range toolNames {
		t, _ := o.toolRegistry.Get(name)
		toolDesc += fmt.Sprintf("- %s: %s\n", name, t.Description())
	}

	prompt += toolDesc
	prompt += `
## 规则
- 系统性地分析题目，不要猜测
- 每次工具调用只执行一个操作
- 仔细分析工具输出，根据结果调整策略
- 当你找到 flag 时，直接输出它（格式如 flag{...}）
- 如果一种方法失败，尝试其他思路`

	return prompt
}

func (o *Orchestrator) buildUserMessage(req SolveRequest) string {
	msg := fmt.Sprintf("题目类型: %s\n", req.ChallengeType)
	msg += fmt.Sprintf("题目描述: %s\n", req.Description)
	if req.Target != "" {
		msg += fmt.Sprintf("目标地址: %s\n", req.Target)
	}
	if len(req.Files) > 0 {
		msg += "相关文件:\n"
		for _, f := range req.Files {
			msg += fmt.Sprintf("- %s\n", f)
		}
	}
	msg += "\n请分析这道题目并找到 flag。"
	return msg
}

// extractFlag searches for flag patterns using configured regexes.
func (o *Orchestrator) extractFlag(text string) string {
	// Use configured regex patterns
	for _, re := range o.flagPatterns {
		if match := re.FindString(text); match != "" {
			return match
		}
	}

	// Fallback: simple patterns
	fallbacks := []string{"flag{", "CTF{", "picoCTF{", "FLAG{"}
	for _, prefix := range fallbacks {
		start := indexOf(text, prefix)
		if start == -1 {
			continue
		}
		end := indexOf(text[start:], "}")
		if end == -1 {
			continue
		}
		return text[start : start+end+1]
	}

	return ""
}

// buildPayload is kept for backward compatibility
func buildPayload(toolUses []llm.ContentBlock) json.RawMessage {
	data, _ := json.Marshal(toolUses)
	return data
}
