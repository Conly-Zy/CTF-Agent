import { useEffect, useState } from 'react'
import {
  api,
  type ConversationMessage,
  type PlanSuggestResult,
  type Session,
  type Subtask,
  type ToolCall,
  type ToolCallStats,
} from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { challengeTypeLabel, statusLabel, formatDuration, formatDate } from '@/lib/utils'
import { Download, RefreshCw, Sparkles, Wand2 } from 'lucide-react'

type BadgeVariant = 'default' | 'secondary' | 'success' | 'destructive' | 'warning' | 'outline'

function planStatusLabel(status: string): string {
  const map: Record<string, string> = {
    planned: '计划中',
    running: '执行中',
    covered: '已覆盖',
    success: '成功',
    needs_review: '需复核',
    skipped: '跳过',
    failed: '失败',
  }
  return map[status] || status
}

function planStatusVariant(status: string): BadgeVariant {
  if (status === 'success' || status === 'covered') return 'success'
  if (status === 'failed') return 'destructive'
  if (status === 'needs_review' || status === 'running') return 'warning'
  if (status === 'planned') return 'outline'
  return 'secondary'
}

function toolStatusVariant(status: string): BadgeVariant {
  if (status === 'success') return 'success'
  if (status === 'failed' || status === 'error') return 'destructive'
  if (status === 'running') return 'warning'
  return 'secondary'
}

function truncate(value: string, limit = 180): string {
  if (!value) return ''
  return value.length > limit ? `${value.slice(0, limit)}...` : value
}

export default function Sessions() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [typeFilter, setTypeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [selected, setSelected] = useState<Session | null>(null)
  const [messages, setMessages] = useState<ConversationMessage[]>([])
  const [plan, setPlan] = useState<Subtask[]>([])
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([])
  const [toolStats, setToolStats] = useState<ToolCallStats | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [planAction, setPlanAction] = useState('')
  const [planNotice, setPlanNotice] = useState('')
  const [suggestion, setSuggestion] = useState<PlanSuggestResult | null>(null)

  const load = () => {
    api.getSessions(50, 0, typeFilter || undefined, statusFilter || undefined).then(setSessions).catch(console.error)
  }

  useEffect(load, [typeFilter, statusFilter])

  const openDetail = async (s: Session) => {
    setSelected(s)
    setDetailLoading(true)
    setPlanNotice('')
    setSuggestion(null)

    const [msgResult, planResult, callsResult, statsResult] = await Promise.allSettled([
      api.getSessionMessages(s.id),
      api.getSessionPlan(s.id),
      api.getSessionToolCalls(s.id, 200),
      api.getSessionToolCallStats(s.id),
    ])

    setMessages(msgResult.status === 'fulfilled' ? msgResult.value : [])
    setPlan(planResult.status === 'fulfilled' ? planResult.value : [])
    setToolCalls(callsResult.status === 'fulfilled' ? callsResult.value : [])
    setToolStats(statsResult.status === 'fulfilled' ? statsResult.value : null)
    setDetailLoading(false)
  }

  const closeDetail = () => {
    setSelected(null)
    setMessages([])
    setPlan([])
    setToolCalls([])
    setToolStats(null)
    setPlanNotice('')
    setSuggestion(null)
    setPlanAction('')
  }

  const handleGeneratePlan = async (useLLM: boolean) => {
    if (!selected) return
    setPlanAction(useLLM ? 'generate_llm' : 'generate')
    setPlanNotice('')
    try {
      const result = await api.generatePlan(selected.id, { llm: useLLM, replace: true })
      setPlan(result.plan)
      setSuggestion(null)
      setPlanNotice(`已重新生成 ${result.created} 个计划项（${result.source}）`)
    } catch (err: unknown) {
      setPlanNotice(err instanceof Error ? err.message : '重新生成计划失败')
    } finally {
      setPlanAction('')
    }
  }

  const handleRefinePlan = async () => {
    if (!selected) return
    setPlanAction('refine')
    setPlanNotice('')
    try {
      const result = await api.refinePlan(selected.id)
      setPlan(result)
      setSuggestion(null)
      setPlanNotice('已根据当前会话结果刷新计划状态')
    } catch (err: unknown) {
      setPlanNotice(err instanceof Error ? err.message : '刷新计划状态失败')
    } finally {
      setPlanAction('')
    }
  }

  const handleSuggestPatch = async (useLLM: boolean, apply: boolean) => {
    if (!selected) return
    setPlanAction(`${useLLM ? 'llm_' : ''}${apply ? 'apply' : 'suggest'}`)
    setPlanNotice('')
    try {
      const result = await api.suggestPlanPatch(selected.id, { llm: useLLM, apply })
      setSuggestion(result)
      if (result.plan) setPlan(result.plan)
      const opCount = result.suggestion.patch.operations.length
      setPlanNotice(
        result.applied
          ? `已应用 ${opCount} 个 Patch 操作（${result.suggestion.source}）`
          : `已生成 ${opCount} 个 Patch 建议（${result.suggestion.source}）`
      )
    } catch (err: unknown) {
      setPlanNotice(err instanceof Error ? err.message : '生成计划 Patch 失败')
    } finally {
      setPlanAction('')
    }
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-semibold">会话记录</h1>

      {/* Filters */}
      <div className="flex gap-3">
        <div className="w-40">
          <Select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
            <option value="">全部类型</option>
            <option value="web">Web安全</option>
            <option value="pwn">二进制漏洞</option>
            <option value="crypto">密码学</option>
            <option value="reverse">逆向工程</option>
          </Select>
        </div>
        <div className="w-40">
          <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">全部状态</option>
            <option value="success">成功</option>
            <option value="failed">失败</option>
            <option value="solving">解题中</option>
          </Select>
        </div>
      </div>

      {/* Session list */}
      {sessions.length === 0 ? (
        <p className="text-muted-foreground">暂无会话记录</p>
      ) : (
        <div className="space-y-3">
          {sessions.map((s) => (
            <Card
              key={s.id}
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => openDetail(s)}
            >
              <CardContent className="p-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Badge variant={s.status === 'success' ? 'success' : s.status === 'failed' ? 'destructive' : 'warning'}>
                    {statusLabel(s.status)}
                  </Badge>
                  <Badge variant="secondary">{challengeTypeLabel(s.challenge_type)}</Badge>
                  <span className="text-sm font-medium">#{s.id}</span>
                  <span className="text-sm text-muted-foreground truncate max-w-md">{s.description}</span>
                </div>
                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                  {s.iterations > 0 && <span>{s.iterations} 轮迭代</span>}
                  {s.duration_ms > 0 && <span>{formatDuration(s.duration_ms)}</span>}
                  <span>{formatDate(s.created_at)}</span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Detail dialog */}
      <Dialog open={!!selected} onOpenChange={(o) => { if (!o) closeDetail() }}>
        {selected && (
          <>
            <DialogHeader>
              <div className="flex items-center justify-between">
                <DialogTitle>会话 #{selected.id} - {challengeTypeLabel(selected.challenge_type)}</DialogTitle>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => api.exportSession(selected.id, 'markdown')}
                    title="导出 Markdown"
                  >
                    <Download className="h-4 w-4 mr-1" /> MD
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => api.exportSession(selected.id, 'json')}
                    title="导出 JSON"
                  >
                    <Download className="h-4 w-4 mr-1" /> JSON
                  </Button>
                </div>
              </div>
            </DialogHeader>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div><span className="text-muted-foreground">状态:</span> {statusLabel(selected.status)}</div>
                <div><span className="text-muted-foreground">迭代:</span> {selected.iterations}</div>
                <div><span className="text-muted-foreground">目标:</span> {selected.target || '-'}</div>
                <div><span className="text-muted-foreground">耗时:</span> {selected.duration_ms > 0 ? formatDuration(selected.duration_ms) : '-'}</div>
              </div>
              <div className="text-sm">
                <span className="text-muted-foreground">描述:</span> {selected.description}
              </div>
              {selected.flag && (
                <div className="p-2 bg-green-50 rounded border text-sm">
                  <span className="font-medium text-green-700">Flag: </span>
                  <code>{selected.flag}</code>
                </div>
              )}
              {selected.error && (
                <div className="p-2 bg-red-50 rounded border text-sm text-red-600">{selected.error}</div>
              )}
              {detailLoading && (
                <div className="text-sm text-muted-foreground">加载计划与证据中...</div>
              )}

              {/* Plan */}
              <div className="rounded-md border p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <h3 className="text-sm font-medium">执行计划</h3>
                    <p className="text-xs text-muted-foreground mt-1">
                      来自后端持久化的 Planner 子任务，可重生成、refine 或应用 Patch。
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-1">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleGeneratePlan(false)}
                      disabled={!!planAction}
                    >
                      <RefreshCw className="h-3.5 w-3.5 mr-1" /> 重生成
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleGeneratePlan(true)}
                      disabled={!!planAction}
                    >
                      <Sparkles className="h-3.5 w-3.5 mr-1" /> LLM 生成
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleRefinePlan}
                      disabled={!!planAction}
                    >
                      Refine
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleSuggestPatch(false, false)}
                      disabled={!!planAction}
                    >
                      <Wand2 className="h-3.5 w-3.5 mr-1" /> 建议 Patch
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleSuggestPatch(false, true)}
                      disabled={!!planAction}
                    >
                      应用建议
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleSuggestPatch(true, true)}
                      disabled={!!planAction}
                    >
                      LLM 应用
                    </Button>
                  </div>
                </div>
                {planAction && (
                  <p className="text-xs text-muted-foreground">计划操作执行中...</p>
                )}
                {planNotice && (
                  <div className="rounded bg-muted p-2 text-xs text-muted-foreground">{planNotice}</div>
                )}
                {suggestion && (
                  <div className="rounded bg-muted/60 p-2 text-xs">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge variant="secondary">{suggestion.suggestion.source}</Badge>
                      <span>{suggestion.suggestion.patch.message || 'Patch suggestion'}</span>
                    </div>
                    {suggestion.suggestion.error && (
                      <p className="text-red-600 mb-1">LLM fallback: {suggestion.suggestion.error}</p>
                    )}
                    {suggestion.suggestion.patch.operations.length === 0 ? (
                      <p className="text-muted-foreground">暂无操作建议。</p>
                    ) : (
                      <div className="space-y-1">
                        {suggestion.suggestion.patch.operations.map((op, index) => (
                          <div key={`${op.op}-${index}`} className="font-mono text-[11px]">
                            {index + 1}. {op.op}
                            {op.id ? ` #${op.id}` : ''}
                            {op.after_id !== undefined ? ` after #${op.after_id}` : ''}
                            {op.title ? ` · ${op.title}` : ''}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
                <div className="space-y-2 max-h-[280px] overflow-auto">
                  {plan.length === 0 ? (
                    <p className="text-sm text-muted-foreground">暂无计划项，可点击“重生成”。</p>
                  ) : (
                    plan.map((item, index) => (
                      <div key={item.id || `${item.task_id}-${index}`} className="rounded-md border p-2 text-sm">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="outline">#{item.sort_order || index + 1}</Badge>
                          <Badge variant={planStatusVariant(item.status)}>
                            {planStatusLabel(item.status)}
                          </Badge>
                          <span className="font-medium">{item.title || '(未命名计划项)'}</span>
                        </div>
                        {item.description && (
                          <p className="mt-1 text-xs text-muted-foreground whitespace-pre-wrap">
                            {item.description}
                          </p>
                        )}
                        {item.result && (
                          <p className="mt-1 text-xs text-green-700 whitespace-pre-wrap">
                            结果: {truncate(item.result)}
                          </p>
                        )}
                        {item.error && (
                          <p className="mt-1 text-xs text-red-600 whitespace-pre-wrap">
                            错误: {truncate(item.error)}
                          </p>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>

              {/* Tool evidence */}
              <div className="rounded-md border p-3 space-y-3">
                <div>
                  <h3 className="text-sm font-medium">工具调用证据</h3>
                  <p className="text-xs text-muted-foreground mt-1">
                    记录 PentAGI Action/ToolCall 风格的调用链，用于报告与计划 refinement。
                  </p>
                </div>
                {toolStats && (
                  <div className="grid grid-cols-3 gap-2 text-xs">
                    <div className="rounded bg-muted p-2">
                      <div className="text-muted-foreground">总调用</div>
                      <div className="text-lg font-semibold">{toolStats.total_calls}</div>
                    </div>
                    <div className="rounded bg-muted p-2">
                      <div className="text-muted-foreground">成功/失败</div>
                      <div className="text-lg font-semibold">
                        {toolStats.success_calls}/{toolStats.failed_calls}
                      </div>
                    </div>
                    <div className="rounded bg-muted p-2">
                      <div className="text-muted-foreground">平均耗时</div>
                      <div className="text-lg font-semibold">
                        {formatDuration(Math.round(toolStats.avg_duration_ms || 0))}
                      </div>
                    </div>
                  </div>
                )}
                <div className="space-y-2 max-h-[220px] overflow-auto">
                  {toolCalls.length === 0 ? (
                    <p className="text-sm text-muted-foreground">暂无工具调用记录</p>
                  ) : (
                    toolCalls.slice(0, 20).map((call) => (
                      <div key={call.id} className="rounded-md border p-2 text-sm">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant={toolStatusVariant(call.status)}>{call.status || 'unknown'}</Badge>
                          <Badge variant="outline">{call.tool_name}</Badge>
                          {call.agent_name && <span className="text-xs text-muted-foreground">{call.agent_name}</span>}
                          {call.duration_ms > 0 && (
                            <span className="text-xs text-muted-foreground">
                              {formatDuration(call.duration_ms)}
                            </span>
                          )}
                        </div>
                        {(call.error || call.output || call.input) && (
                          <pre className="mt-1 whitespace-pre-wrap break-all text-xs text-muted-foreground">
                            {truncate(call.error || call.output || call.input)}
                          </pre>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>

              {/* Conversation */}
              <div>
                <h3 className="text-sm font-medium mb-2">对话记录</h3>
                <div className="space-y-2 max-h-[400px] overflow-auto">
                  {messages.length === 0 ? (
                    <p className="text-sm text-muted-foreground">无对话记录</p>
                  ) : (
                    messages.map((m) => (
                      <div key={m.id} className="border rounded-md p-2 text-sm">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant={m.role === 'assistant' ? 'default' : 'secondary'}>
                            {m.role === 'assistant' ? 'AI' : '用户'}
                          </Badge>
                          {m.tool_name && <Badge variant="outline">{m.tool_name}</Badge>}
                        </div>
                        <div className="whitespace-pre-wrap text-muted-foreground">
                          {m.content || m.tool_input || '(空)'}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </>
        )}
      </Dialog>
    </div>
  )
}
