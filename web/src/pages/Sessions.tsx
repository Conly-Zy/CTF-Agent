import { useEffect, useState } from 'react'
import { api, Session, ConversationMessage } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { challengeTypeLabel, statusLabel, formatDuration, formatDate } from '@/lib/utils'

export default function Sessions() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [typeFilter, setTypeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [selected, setSelected] = useState<Session | null>(null)
  const [messages, setMessages] = useState<ConversationMessage[]>([])

  const load = () => {
    api.getSessions(50, 0, typeFilter || undefined, statusFilter || undefined).then(setSessions).catch(console.error)
  }

  useEffect(load, [typeFilter, statusFilter])

  const openDetail = async (s: Session) => {
    setSelected(s)
    try {
      const msgs = await api.getSessionMessages(s.id)
      setMessages(msgs)
    } catch {
      setMessages([])
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
      <Dialog open={!!selected} onOpenChange={(o) => { if (!o) setSelected(null) }}>
        {selected && (
          <>
            <DialogHeader>
              <DialogTitle>会话 #{selected.id} - {challengeTypeLabel(selected.challenge_type)}</DialogTitle>
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
