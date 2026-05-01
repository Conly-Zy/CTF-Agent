const BASE = ''

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `请求失败: ${res.status}`)
  }
  return res.json()
}

// Dashboard
export interface DashboardData {
  stats: {
    total_sessions: number
    success_sessions: number
    failed_sessions: number
    by_type: Record<string, number>
    avg_duration_ms: number
  }
  sessions: Session[]
}

export interface Session {
  id: number
  challenge_type: string
  description: string
  target: string
  files: string
  status: string
  flag: string
  iterations: number
  duration_ms: number
  error: string
  created_at: string
  completed_at: string | null
}

export interface ConversationMessage {
  id: number
  session_id: number
  role: string
  content: string
  tool_name: string
  tool_input: string
  created_at: string
}

export interface KnowledgeItem {
  id: number
  session_id: number
  title: string
  content: string
  type: string
  created_at: string
}

export interface Tag {
  id: number
  name: string
}

export interface ConfigData {
  anthropic: { api_key: string; model: string }
  agent: { max_iterations: number; timeout_seconds: number; verbose: boolean }
  sandbox: { enabled: boolean; image: string; timeout_seconds: number; network_mode: string }
  flag: { patterns: string[] }
  submit: { enabled: boolean; url: string; method: string; field: string }
}

export const api = {
  getDashboard: () => request<DashboardData>('/api/dashboard'),

  getSessions: (limit = 20, offset = 0, type?: string, status?: string) => {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (type) params.set('type', type)
    if (status) params.set('status', status)
    return request<Session[]>(`/api/sessions?${params}`)
  },

  getSession: (id: number) => request<Session>(`/api/sessions/${id}`),

  getSessionMessages: (id: number) => request<ConversationMessage[]>(`/api/sessions/${id}/messages`),

  solve: (data: { challenge_type: string; description: string; target: string; files: string[] }) =>
    request<{ session_id: number; status: string }>('/api/solve', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  stopSession: (id: number) =>
    request<{ ok: boolean }>(`/api/sessions/${id}/stop`, { method: 'POST' }),

  pauseSession: (id: number) =>
    request<{ ok: boolean }>(`/api/sessions/${id}/pause`, { method: 'POST' }),

  resumeSession: (id: number) =>
    request<{ ok: boolean }>(`/api/sessions/${id}/resume`, { method: 'POST' }),

  injectMessage: (id: number, message: string) =>
    request<{ ok: boolean }>(`/api/sessions/${id}/inject`, {
      method: 'POST',
      body: JSON.stringify({ message }),
    }),

  getKnowledge: (limit = 20, offset = 0, type?: string) => {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (type) params.set('type', type)
    return request<KnowledgeItem[]>(`/api/knowledge?${params}`)
  },

  getKnowledgeItem: (id: number) =>
    request<{ knowledge: KnowledgeItem; tags: Tag[] }>(`/api/knowledge/${id}`),

  searchKnowledge: (q: string, limit = 20) =>
    request<KnowledgeItem[]>(`/api/knowledge/search?q=${encodeURIComponent(q)}&limit=${limit}`),

  getTags: () => request<Tag[]>('/api/tags'),

  getConfig: () => request<ConfigData>('/api/config'),

  updateConfig: (data: ConfigData) =>
    request<{ ok: boolean }>('/api/config', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
}
