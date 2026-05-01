export function cn(...classes: (string | undefined | false | null)[]) {
  return classes.filter(Boolean).join(' ')
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  return `${m}m ${s}s`
}

export function formatDate(d: string | Date): string {
  const date = new Date(d)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function challengeTypeLabel(type: string): string {
  const map: Record<string, string> = {
    web: 'Web安全',
    pwn: '二进制漏洞',
    crypto: '密码学',
    reverse: '逆向工程',
  }
  return map[type] || type
}

export function statusLabel(status: string): string {
  const map: Record<string, string> = {
    solving: '解题中',
    success: '成功',
    failed: '失败',
    pending: '等待中',
  }
  return map[status] || status
}

export function knowledgeTypeLabel(type: string): string {
  const map: Record<string, string> = {
    vulnerability: '漏洞',
    exploit: '利用技术',
    technique: '技巧',
    analysis: '分析',
  }
  return map[type] || type
}
