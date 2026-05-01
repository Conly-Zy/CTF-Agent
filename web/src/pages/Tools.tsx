import { useEffect, useState } from 'react'
import { api, ToolInfo, ToolTestResult } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Play, Wrench } from 'lucide-react'

export default function Tools() {
  const [tools, setTools] = useState<ToolInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<ToolInfo | null>(null)
  const [testInput, setTestInput] = useState('{}')
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<ToolTestResult | null>(null)

  useEffect(() => {
    api.getTools().then(setTools).catch(console.error).finally(() => setLoading(false))
  }, [])

  const handleTest = async () => {
    if (!selected) return
    setTesting(true)
    setTestResult(null)
    try {
      const input = JSON.parse(testInput)
      const result = await api.testTool(selected.name, input)
      setTestResult(result)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '测试失败'
      setTestResult({ output: '', error: msg })
    } finally {
      setTesting(false)
    }
  }

  const openDetail = (tool: ToolInfo) => {
    setSelected(tool)
    setTestInput('{}')
    setTestResult(null)
  }

  // Group tools by category (prefix before _)
  const grouped = tools.reduce<Record<string, ToolInfo[]>>((acc, tool) => {
    const parts = tool.name.split('_')
    const category = parts.length > 1 ? parts[0] : 'other'
    if (!acc[category]) acc[category] = []
    acc[category].push(tool)
    return acc
  }, {})

  const categoryLabels: Record<string, string> = {
    http: 'HTTP 请求',
    dir: '目录扫描',
    binary: '二进制分析',
    disasm: '反汇编',
    pattern: '模式生成',
    encode: '编码解码',
    hash: '哈希识别',
    math: '数学计算',
    strings: '字符串提取',
    hexdump: '十六进制',
    entropy: '熵分析',
    other: '其他',
  }

  if (loading) {
    return <div className="p-6 text-muted-foreground">加载中...</div>
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-semibold">工具管理</h1>
        <Badge variant="secondary">{tools.length} 个工具</Badge>
      </div>

      {tools.length === 0 ? (
        <p className="text-muted-foreground">暂无已注册工具</p>
      ) : (
        Object.entries(grouped).map(([category, categoryTools]) => (
          <div key={category} className="space-y-3">
            <h2 className="text-lg font-medium">
              {categoryLabels[category] || category}
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {categoryTools.map((tool) => (
                <Card
                  key={tool.name}
                  className="cursor-pointer hover:shadow-md transition-shadow"
                  onClick={() => openDetail(tool)}
                >
                  <CardHeader className="pb-2">
                    <div className="flex items-center gap-2">
                      <Wrench className="h-4 w-4 text-muted-foreground" />
                      <CardTitle className="text-sm">{tool.name}</CardTitle>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <p className="text-xs text-muted-foreground line-clamp-2">
                      {tool.description}
                    </p>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        ))
      )}

      {/* Tool detail dialog */}
      <Dialog open={!!selected} onOpenChange={(o) => { if (!o) setSelected(null) }}>
        {selected && (
          <>
            <DialogHeader>
              <DialogTitle>{selected.name}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium mb-1">描述</h3>
                <p className="text-sm text-muted-foreground">{selected.description}</p>
              </div>

              <div>
                <h3 className="text-sm font-medium mb-1">参数 Schema</h3>
                <pre className="p-3 bg-muted rounded-md text-xs overflow-auto max-h-40">
                  {JSON.stringify(selected.input_schema, null, 2)}
                </pre>
              </div>

              <div>
                <h3 className="text-sm font-medium mb-2">测试调用</h3>
                <Textarea
                  value={testInput}
                  onChange={(e) => setTestInput(e.target.value)}
                  placeholder='{"key": "value"}'
                  rows={3}
                  className="font-mono text-sm"
                />
                <Button
                  size="sm"
                  className="mt-2"
                  onClick={handleTest}
                  disabled={testing}
                >
                  <Play className="h-3.5 w-3.5 mr-1" />
                  {testing ? '执行中...' : '执行测试'}
                </Button>
              </div>

              {testResult && (
                <div>
                  <h3 className="text-sm font-medium mb-1">执行结果</h3>
                  {testResult.error ? (
                    <div className="p-3 bg-red-50 rounded-md text-sm text-red-600">
                      {testResult.error}
                    </div>
                  ) : (
                    <pre className="p-3 bg-green-50 rounded-md text-xs overflow-auto max-h-60">
                      {testResult.output}
                    </pre>
                  )}
                </div>
              )}
            </div>
          </>
        )}
      </Dialog>
    </div>
  )
}
