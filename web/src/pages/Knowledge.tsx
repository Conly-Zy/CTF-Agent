import { useEffect, useState } from 'react'
import { api, KnowledgeItem, Tag } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { knowledgeTypeLabel, formatDate } from '@/lib/utils'
import { Search } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

export default function Knowledge() {
  const [items, setItems] = useState<KnowledgeItem[]>([])
  const [typeFilter, setTypeFilter] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selected, setSelected] = useState<KnowledgeItem | null>(null)
  const [selectedTags, setSelectedTags] = useState<Tag[]>([])

  const load = () => {
    api.getKnowledge(50, 0, typeFilter || undefined).then(setItems).catch(console.error)
  }

  useEffect(load, [typeFilter])

  const handleSearch = async () => {
    if (!searchQuery.trim()) {
      load()
      return
    }
    try {
      const results = await api.searchKnowledge(searchQuery)
      setItems(results)
    } catch {
      setItems([])
    }
  }

  const openDetail = async (item: KnowledgeItem) => {
    setSelected(item)
    try {
      const res = await api.getKnowledgeItem(item.id)
      setSelectedTags(res.tags)
    } catch {
      setSelectedTags([])
    }
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-semibold">知识库</h1>

      {/* Search and filters */}
      <div className="flex gap-3">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="搜索知识库..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleSearch() }}
          />
        </div>
        <Button variant="secondary" onClick={handleSearch}>搜索</Button>
        <div className="w-40">
          <Select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setSearchQuery('') }}>
            <option value="">全部类型</option>
            <option value="technique">技巧</option>
            <option value="vulnerability">漏洞</option>
            <option value="exploit">利用技术</option>
            <option value="analysis">分析</option>
          </Select>
        </div>
      </div>

      {/* Knowledge list */}
      {items.length === 0 ? (
        <p className="text-muted-foreground">暂无知识条目</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {items.map((item) => (
            <Card
              key={item.id}
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => openDetail(item)}
            >
              <CardHeader>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">{knowledgeTypeLabel(item.type)}</Badge>
                  <CardTitle className="truncate">{item.title}</CardTitle>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground line-clamp-2">{item.content.slice(0, 150)}...</p>
                <p className="text-xs text-muted-foreground mt-2">{formatDate(item.created_at)}</p>
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
              <DialogTitle>{selected.title}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <Badge variant="secondary">{knowledgeTypeLabel(selected.type)}</Badge>
                {selectedTags.map((t) => (
                  <Badge key={t.id} variant="outline">{t.name}</Badge>
                ))}
              </div>
              <div className="prose prose-sm max-w-none dark:prose-invert">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{selected.content}</ReactMarkdown>
              </div>
            </div>
          </>
        )}
      </Dialog>
    </div>
  )
}
