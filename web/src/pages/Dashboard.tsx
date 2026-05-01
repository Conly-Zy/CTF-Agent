import { useEffect, useState } from 'react'
import { api, DashboardData } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { challengeTypeLabel, statusLabel, formatDuration, formatDate } from '@/lib/utils'
import { Activity, CheckCircle, XCircle, Clock } from 'lucide-react'

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null)

  useEffect(() => {
    api.getDashboard().then(setData).catch(console.error)
  }, [])

  if (!data) {
    return <div className="p-6 text-muted-foreground">加载中...</div>
  }

  const { stats, sessions } = data
  const successRate = stats.total_sessions > 0
    ? ((stats.success_sessions / stats.total_sessions) * 100).toFixed(1)
    : '0'

  const statCards = [
    { title: '总会话数', value: stats.total_sessions, icon: Activity, color: 'text-blue-600' },
    { title: '成功率', value: `${successRate}%`, icon: CheckCircle, color: 'text-green-600' },
    { title: '失败数', value: stats.failed_sessions, icon: XCircle, color: 'text-red-600' },
    { title: '平均耗时', value: formatDuration(stats.avg_duration_ms), icon: Clock, color: 'text-yellow-600' },
  ]

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-semibold">仪表盘</h1>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {statCards.map(({ title, value, icon: Icon, color }) => (
          <Card key={title}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm text-muted-foreground">{title}</CardTitle>
              <Icon className={`h-4 w-4 ${color}`} />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Type breakdown */}
      <Card>
        <CardHeader>
          <CardTitle>题目类型分布</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            {Object.entries(stats.by_type).map(([type, count]) => (
              <div key={type} className="text-center">
                <Badge variant="secondary">{challengeTypeLabel(type)}</Badge>
                <div className="text-2xl font-bold mt-2">{count}</div>
              </div>
            ))}
            {Object.keys(stats.by_type).length === 0 && (
              <p className="text-sm text-muted-foreground col-span-full">暂无数据</p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Recent sessions */}
      <Card>
        <CardHeader>
          <CardTitle>最近会话</CardTitle>
        </CardHeader>
        <CardContent>
          {sessions.length === 0 ? (
            <p className="text-sm text-muted-foreground">暂无会话记录</p>
          ) : (
            <div className="space-y-3">
              {sessions.map((s) => (
                <div key={s.id} className="flex items-center justify-between py-2 border-b last:border-0">
                  <div className="flex items-center gap-3">
                    <Badge variant={s.status === 'success' ? 'success' : s.status === 'failed' ? 'destructive' : 'warning'}>
                      {statusLabel(s.status)}
                    </Badge>
                    <span className="text-sm font-medium">{challengeTypeLabel(s.challenge_type)}</span>
                    <span className="text-sm text-muted-foreground truncate max-w-xs">{s.description}</span>
                  </div>
                  <span className="text-xs text-muted-foreground">{formatDate(s.created_at)}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
