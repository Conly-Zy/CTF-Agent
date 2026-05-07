import { useState, useRef, useEffect, useCallback } from 'react'
import { api, type FlowTemplate, type UploadResult } from '@/lib/api'
import { useWebSocket, WSMessage } from '@/hooks/useWebSocket'
import { challengeTemplates, ChallengeTemplate } from '@/data/templates'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Square, Pause, Play, Send, Upload, X, File, BookTemplate } from 'lucide-react'

interface LogEntry {
  id: number
  type: string
  level?: string
  message?: string
  tool?: string
  result?: string
  content?: string
  flag?: string
  timestamp: string
}

export default function Solve() {
  const [challengeType, setChallengeType] = useState('web')
  const [description, setDescription] = useState('')
  const [target, setTarget] = useState('')
  const [uploadedFiles, setUploadedFiles] = useState<UploadResult[]>([])
  const [uploading, setUploading] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const [solving, setSolving] = useState(false)
  const [paused, setPaused] = useState(false)
  const [sessionId, setSessionId] = useState<number | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [result, setResult] = useState<{ flag?: string; success: boolean; error?: string } | null>(null)
  const [injectMsg, setInjectMsg] = useState('')
  const [showTemplates, setShowTemplates] = useState(false)
  const [planWithLLM, setPlanWithLLM] = useState(false)
  const [flowTemplates, setFlowTemplates] = useState<FlowTemplate[]>([])
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [templateLoading, setTemplateLoading] = useState(false)
  const logEndRef = useRef<HTMLDivElement>(null)
  const logIdRef = useRef(0)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const addLog = (entry: Omit<LogEntry, 'id' | 'timestamp'>) => {
    setLogs((prev) => [
      ...prev,
      {
        ...entry,
        id: ++logIdRef.current,
        timestamp: new Date().toLocaleTimeString('zh-CN'),
      },
    ])
  }

  const { connected } = useWebSocket((msg: WSMessage) => {
    switch (msg.type) {
      case 'log':
        addLog({ type: 'log', level: msg.level, message: msg.message })
        break
      case 'tool_start':
        addLog({ type: 'tool', message: `调用工具: ${msg.tool}` })
        break
      case 'tool_result':
        addLog({ type: 'tool', message: `${msg.tool} 返回结果`, result: msg.result })
        break
      case 'thinking':
        addLog({ type: 'thinking', content: msg.content })
        break
      case 'flag':
        addLog({ type: 'flag', message: `Flag: ${msg.flag}` })
        break
      case 'complete':
        setSolving(false)
        setPaused(false)
        setSessionId(null)
        setResult({
          flag: msg.flag,
          success: msg.success ?? false,
          error: msg.error,
        })
        addLog({
          type: msg.success ? 'success' : 'error',
          message: msg.success ? `解题成功! 耗时 ${msg.duration}` : `解题失败: ${msg.error}`,
        })
        break
    }
  })

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  useEffect(() => {
    let cancelled = false
    setTemplateLoading(true)
    api.getTemplates(challengeType)
      .then((items) => {
        if (cancelled) return
        setFlowTemplates(items)
        setSelectedTemplateId((prev) => (
          prev && items.some((item) => String(item.id) === prev) ? prev : ''
        ))
      })
      .catch(() => {
        if (!cancelled) {
          setFlowTemplates([])
          setSelectedTemplateId('')
        }
      })
      .finally(() => {
        if (!cancelled) setTemplateLoading(false)
      })
    return () => { cancelled = true }
  }, [challengeType])

  const handleFileUpload = useCallback(async (fileList: FileList | File[]) => {
    setUploading(true)
    const newFiles: UploadResult[] = []

    for (const file of Array.from(fileList)) {
      try {
        const result = await api.uploadFile(file)
        newFiles.push(result)
        addLog({ type: 'log', level: 'info', message: `文件已上传: ${file.name}` })
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : '上传失败'
        addLog({ type: 'error', message: `上传 ${file.name} 失败: ${msg}` })
      }
    }

    setUploadedFiles((prev) => [...prev, ...newFiles])
    setUploading(false)
  }, [])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    if (e.dataTransfer.files.length > 0) {
      handleFileUpload(e.dataTransfer.files)
    }
  }, [handleFileUpload])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(true)
  }, [])

  const handleDragLeave = useCallback(() => {
    setDragOver(false)
  }, [])

  const removeFile = (index: number) => {
    setUploadedFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const applyTemplate = (template: ChallengeTemplate) => {
    setChallengeType(template.type)
    setDescription(template.template)
    if (template.targetPlaceholder) {
      setTarget('')
    }
    setShowTemplates(false)
  }

  const filteredTemplates = challengeTemplates.filter((t) => t.type === challengeType)

  const handleSubmit = async () => {
    if (!description.trim()) return
    setSolving(true)
    setPaused(false)
    setSessionId(null)
    setLogs([])
    setResult(null)

    addLog({ type: 'log', level: 'info', message: '提交解题任务...' })

    try {
      const templateId = Number(selectedTemplateId)
      const res = await api.solve({
        challenge_type: challengeType,
        description,
        target,
        files: uploadedFiles.map((f) => f.path),
        plan_with_llm: planWithLLM,
        ...(templateId > 0 ? { template_id: templateId } : {}),
      })
      setSessionId(res.session_id)
      addLog({ type: 'log', level: 'info', message: `会话 #${res.session_id} 已创建，开始解题...` })
      if (typeof res.planned_subtasks === 'number') {
        addLog({
          type: 'log',
          level: 'info',
          message: `初始计划已生成: ${res.planned_subtasks} 个子任务 (${res.plan_source || 'unknown'})`,
        })
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '未知错误'
      addLog({ type: 'error', message: `提交失败: ${msg}` })
      setSolving(false)
    }
  }

  const handleStop = async () => {
    if (!sessionId) return
    try {
      await api.stopSession(sessionId)
    } catch {}
  }

  const handlePause = async () => {
    if (!sessionId) return
    try {
      await api.pauseSession(sessionId)
      setPaused(true)
    } catch {}
  }

  const handleResume = async () => {
    if (!sessionId) return
    try {
      await api.resumeSession(sessionId)
      setPaused(false)
    } catch {}
  }

  const handleInject = async () => {
    if (!sessionId || !injectMsg.trim()) return
    try {
      await api.injectMessage(sessionId, injectMsg.trim())
      addLog({ type: 'log', level: 'info', message: `手动消息已发送: ${injectMsg.trim()}` })
      setInjectMsg('')
    } catch {}
  }

  const logTypeClass: Record<string, string> = {
    log: 'text-blue-700',
    tool: 'text-purple-700',
    thinking: 'text-muted-foreground italic',
    flag: 'text-green-700 font-semibold',
    success: 'text-green-700 font-semibold',
    error: 'text-red-600',
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-semibold">解题</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Form */}
        <Card>
          <CardHeader>
            <CardTitle>题目信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium">题目类型</label>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowTemplates(true)}
                  className="h-7 text-xs"
                >
                  <BookTemplate className="h-3.5 w-3.5 mr-1" /> 使用模板
                </Button>
              </div>
              <Select value={challengeType} onChange={(e) => setChallengeType(e.target.value)}>
                <option value="web">Web安全</option>
                <option value="pwn">二进制漏洞</option>
                <option value="crypto">密码学</option>
                <option value="reverse">逆向工程</option>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">题目描述</label>
              <Textarea
                rows={4}
                placeholder="描述题目内容..."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">目标地址</label>
              <Input
                placeholder="http://example.com 或 host:port"
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              />
            </div>
            <div className="space-y-3 rounded-md border bg-muted/20 p-3">
              <div>
                <div className="text-sm font-medium">PentAGI 风格计划</div>
                <p className="text-xs text-muted-foreground mt-1">
                  创建会话时生成可追踪的 Flow/Task/Subtask 初始计划，后续可在会话详情中 refinement。
                </p>
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="h-4 w-4"
                  checked={planWithLLM}
                  onChange={(e) => setPlanWithLLM(e.target.checked)}
                  disabled={solving}
                />
                使用 LLM 生成初始计划（失败时自动回退模板解析）
              </label>
              <div className="space-y-2">
                <label className="text-sm font-medium">流程模板</label>
                <Select
                  value={selectedTemplateId}
                  onChange={(e) => setSelectedTemplateId(e.target.value)}
                  disabled={solving || templateLoading || flowTemplates.length === 0}
                >
                  <option value="">
                    {templateLoading
                      ? '加载模板中...'
                      : flowTemplates.length > 0
                        ? '自动选择默认模板'
                        : '暂无后端流程模板'}
                  </option>
                  {flowTemplates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.title}
                    </option>
                  ))}
                </Select>
                {selectedTemplateId && (
                  <p className="text-xs text-muted-foreground">
                    {flowTemplates.find((template) => String(template.id) === selectedTemplateId)?.description}
                  </p>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">题目文件</label>
              <div
                className={`border-2 border-dashed rounded-lg p-4 text-center cursor-pointer transition-colors ${
                  dragOver ? 'border-primary bg-primary/5' : 'border-muted-foreground/25 hover:border-muted-foreground/50'
                }`}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onClick={() => fileInputRef.current?.click()}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  className="hidden"
                  onChange={(e) => e.target.files && handleFileUpload(e.target.files)}
                />
                <Upload className="h-6 w-6 mx-auto mb-2 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  {uploading ? '上传中...' : '拖拽文件到此处或点击选择'}
                </p>
                <p className="text-xs text-muted-foreground mt-1">
                  支持二进制文件、脚本、PCAP 等
                </p>
              </div>

              {/* Uploaded files list */}
              {uploadedFiles.length > 0 && (
                <div className="space-y-1 mt-2">
                  {uploadedFiles.map((file, index) => (
                    <div key={index} className="flex items-center justify-between py-1 px-2 bg-muted rounded text-sm">
                      <div className="flex items-center gap-2 truncate">
                        <File className="h-3.5 w-3.5 shrink-0" />
                        <span className="truncate">{file.name}</span>
                        <span className="text-xs text-muted-foreground">
                          {(file.size / 1024).toFixed(1)} KB
                        </span>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-5 w-5"
                        onClick={(e) => { e.stopPropagation(); removeFile(index) }}
                      >
                        <X className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <Button onClick={handleSubmit} disabled={solving || !description.trim()} className="w-full">
              {solving ? '解题中...' : '开始解题'}
            </Button>
          </CardContent>
        </Card>

        {/* Output */}
        <Card className="flex flex-col">
          <CardHeader className="flex flex-row items-center justify-between space-y-0">
            <CardTitle>输出日志</CardTitle>
            <div className="flex items-center gap-2">
              <Badge variant={connected ? 'success' : 'destructive'}>
                {connected ? '已连接' : '未连接'}
              </Badge>
              {solving && (
                <Badge variant={paused ? 'warning' : 'default'}>
                  {paused ? '已暂停' : '解题中'}
                </Badge>
              )}
            </div>
          </CardHeader>
          <CardContent className="flex-1 flex flex-col gap-3">
            {/* Control buttons */}
            {solving && sessionId && (
              <div className="flex items-center gap-2">
                {paused ? (
                  <Button size="sm" variant="outline" onClick={handleResume}>
                    <Play className="h-3.5 w-3.5 mr-1" /> 恢复
                  </Button>
                ) : (
                  <Button size="sm" variant="outline" onClick={handlePause}>
                    <Pause className="h-3.5 w-3.5 mr-1" /> 暂停
                  </Button>
                )}
                <Button size="sm" variant="destructive" onClick={handleStop}>
                  <Square className="h-3.5 w-3.5 mr-1" /> 停止
                </Button>
                <span className="text-xs text-muted-foreground ml-2">会话 #{sessionId}</span>
              </div>
            )}

            {/* Log output */}
            <div className="flex-1 bg-[#1e1e1e] rounded-md p-3 overflow-auto font-mono text-xs min-h-[300px] max-h-[500px]">
              {logs.length === 0 ? (
                <div className="text-gray-500">等待开始...</div>
              ) : (
                logs.map((log) => (
                  <div key={log.id} className={`py-0.5 ${logTypeClass[log.type] || 'text-gray-300'}`}>
                    <span className="text-gray-500 mr-2">[{log.timestamp}]</span>
                    {log.message || log.content || ''}
                    {log.result && (
                      <pre className="text-gray-400 mt-1 whitespace-pre-wrap break-all max-h-32 overflow-auto">
                        {log.result.length > 500 ? log.result.slice(0, 500) + '...' : log.result}
                      </pre>
                    )}
                  </div>
                ))
              )}
              <div ref={logEndRef} />
            </div>

            {/* Message injection */}
            {solving && sessionId && (
              <div className="flex gap-2">
                <Input
                  placeholder="向 Agent 发送消息..."
                  value={injectMsg}
                  onChange={(e) => setInjectMsg(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleInject() }}
                  className="text-sm"
                />
                <Button size="icon" variant="outline" onClick={handleInject} disabled={!injectMsg.trim()}>
                  <Send className="h-4 w-4" />
                </Button>
              </div>
            )}

            {/* Result */}
            {result && (
              <div className="p-3 rounded-md border">
                {result.success && result.flag ? (
                  <div>
                    <div className="text-sm font-medium text-green-700 mb-1">Flag 已找到</div>
                    <code className="block p-2 bg-green-50 rounded text-sm break-all">{result.flag}</code>
                  </div>
                ) : (
                  <div className="text-sm text-red-600">{result.error || '解题失败'}</div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Template selection dialog */}
      <Dialog open={showTemplates} onOpenChange={setShowTemplates}>
        <DialogHeader>
          <DialogTitle>选择题目模板</DialogTitle>
        </DialogHeader>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-[400px] overflow-auto">
          {filteredTemplates.map((template) => (
            <Card
              key={template.id}
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => applyTemplate(template)}
            >
              <CardContent className="p-3">
                <h3 className="text-sm font-medium">{template.name}</h3>
                <p className="text-xs text-muted-foreground mt-1">{template.description}</p>
                {template.tips && (
                  <div className="flex flex-wrap gap-1 mt-2">
                    {template.tips.slice(0, 2).map((tip, i) => (
                      <Badge key={i} variant="secondary" className="text-[10px]">
                        {tip.length > 20 ? tip.slice(0, 20) + '...' : tip}
                      </Badge>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
          {filteredTemplates.length === 0 && (
            <p className="text-sm text-muted-foreground col-span-2 text-center py-4">
              暂无该类型的模板
            </p>
          )}
        </div>
      </Dialog>
    </div>
  )
}
