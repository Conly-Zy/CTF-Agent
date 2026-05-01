import { useEffect, useState } from 'react'
import { api, ConfigData } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Save, RotateCcw, Eye, EyeOff, Plus, X } from 'lucide-react'

export default function Settings() {
  const [config, setConfig] = useState<ConfigData | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [showKey, setShowKey] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getConfig().then(setConfig).catch((e) => setError(e.message))
  }, [])

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await api.updateConfig(config)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    try {
      const fresh = await api.getConfig()
      setConfig(fresh)
      setSaved(false)
      setError('')
    } catch {}
  }

  if (!config) {
    return <div className="p-6 text-muted-foreground">{error || '加载中...'}</div>
  }

  const addPattern = () => {
    setConfig({
      ...config,
      flag: { ...config.flag, patterns: [...config.flag.patterns, ''] },
    })
  }

  const removePattern = (i: number) => {
    const patterns = [...config.flag.patterns]
    patterns.splice(i, 1)
    setConfig({ ...config, flag: { ...config.flag, patterns } })
  }

  const updatePattern = (i: number, val: string) => {
    const patterns = [...config.flag.patterns]
    patterns[i] = val
    setConfig({ ...config, flag: { ...config.flag, patterns } })
  }

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">设置</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={handleReset}>
            <RotateCcw className="h-4 w-4 mr-1" /> 重载
          </Button>
          <Button size="sm" onClick={handleSave} disabled={saving}>
            <Save className="h-4 w-4 mr-1" /> {saving ? '保存中...' : saved ? '已保存' : '保存'}
          </Button>
        </div>
      </div>

      {error && (
        <div className="p-3 rounded-md bg-destructive/10 text-destructive text-sm">{error}</div>
      )}
      {saved && (
        <div className="p-3 rounded-md bg-green-50 text-green-700 text-sm">配置已保存并生效</div>
      )}

      {/* Anthropic */}
      <Card>
        <CardHeader>
          <CardTitle>Anthropic API</CardTitle>
          <CardDescription>配置 Claude API 连接参数</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">API Key</label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Input
                  type={showKey ? 'text' : 'password'}
                  value={config.anthropic.api_key}
                  onChange={(e) => setConfig({ ...config, anthropic: { ...config.anthropic, api_key: e.target.value } })}
                  placeholder="sk-ant-..."
                />
                <button
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowKey(!showKey)}
                >
                  {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">模型</label>
            <Select
              value={config.anthropic.model}
              onChange={(e) => setConfig({ ...config, anthropic: { ...config.anthropic, model: e.target.value } })}
            >
              <option value="claude-opus-4-7">Claude Opus 4.7</option>
              <option value="claude-sonnet-4-6">Claude Sonnet 4.6</option>
              <option value="claude-haiku-4-5-20251001">Claude Haiku 4.5</option>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Agent */}
      <Card>
        <CardHeader>
          <CardTitle>Agent 参数</CardTitle>
          <CardDescription>控制 AI 代理的解题行为</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">最大迭代次数</label>
              <Input
                type="number"
                value={config.agent.max_iterations}
                onChange={(e) => setConfig({
                  ...config,
                  agent: { ...config.agent, max_iterations: parseInt(e.target.value) || 50 },
                })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">超时时间 (秒)</label>
              <Input
                type="number"
                value={config.agent.timeout_seconds}
                onChange={(e) => setConfig({
                  ...config,
                  agent: { ...config.agent, timeout_seconds: parseInt(e.target.value) || 600 },
                })}
              />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="verbose"
              checked={config.agent.verbose}
              onChange={(e) => setConfig({ ...config, agent: { ...config.agent, verbose: e.target.checked } })}
              className="h-4 w-4 rounded border"
            />
            <label htmlFor="verbose" className="text-sm">详细日志</label>
          </div>
        </CardContent>
      </Card>

      {/* Sandbox */}
      <Card>
        <CardHeader>
          <CardTitle>沙箱</CardTitle>
          <CardDescription>Docker 沙箱执行环境配置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="sandbox-enabled"
              checked={config.sandbox.enabled}
              onChange={(e) => setConfig({ ...config, sandbox: { ...config.sandbox, enabled: e.target.checked } })}
              className="h-4 w-4 rounded border"
            />
            <label htmlFor="sandbox-enabled" className="text-sm">启用沙箱</label>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">镜像名称</label>
              <Input
                value={config.sandbox.image}
                onChange={(e) => setConfig({ ...config, sandbox: { ...config.sandbox, image: e.target.value } })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">超时时间 (秒)</label>
              <Input
                type="number"
                value={config.sandbox.timeout_seconds}
                onChange={(e) => setConfig({
                  ...config,
                  sandbox: { ...config.sandbox, timeout_seconds: parseInt(e.target.value) || 60 },
                })}
              />
            </div>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">网络模式</label>
            <Select
              value={config.sandbox.network_mode}
              onChange={(e) => setConfig({ ...config, sandbox: { ...config.sandbox, network_mode: e.target.value } })}
            >
              <option value="bridge">bridge</option>
              <option value="host">host</option>
              <option value="none">none</option>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Flag patterns */}
      <Card>
        <CardHeader>
          <CardTitle>Flag 匹配规则</CardTitle>
          <CardDescription>用于识别解题结果中的 Flag 格式 (正则表达式)</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {config.flag.patterns.map((p, i) => (
            <div key={i} className="flex gap-2">
              <Input
                value={p}
                onChange={(e) => updatePattern(i, e.target.value)}
                placeholder="flag\{[^}]+\}"
                className="font-mono text-sm"
              />
              <Button variant="ghost" size="icon" onClick={() => removePattern(i)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          ))}
          <Button variant="outline" size="sm" onClick={addPattern}>
            <Plus className="h-4 w-4 mr-1" /> 添加规则
          </Button>
        </CardContent>
      </Card>

      {/* Submit */}
      <Card>
        <CardHeader>
          <CardTitle>自动提交</CardTitle>
          <CardDescription>找到 Flag 后自动提交到平台</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="submit-enabled"
              checked={config.submit.enabled}
              onChange={(e) => setConfig({ ...config, submit: { ...config.submit, enabled: e.target.checked } })}
              className="h-4 w-4 rounded border"
            />
            <label htmlFor="submit-enabled" className="text-sm">启用自动提交</label>
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">提交 URL</label>
            <Input
              value={config.submit.url}
              onChange={(e) => setConfig({ ...config, submit: { ...config.submit, url: e.target.value } })}
              placeholder="http://ctf-platform/api/submit"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">请求方法</label>
              <Select
                value={config.submit.method}
                onChange={(e) => setConfig({ ...config, submit: { ...config.submit, method: e.target.value } })}
              >
                <option value="POST">POST</option>
                <option value="GET">GET</option>
                <option value="PUT">PUT</option>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">字段名</label>
              <Input
                value={config.submit.field}
                onChange={(e) => setConfig({ ...config, submit: { ...config.submit, field: e.target.value } })}
              />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
