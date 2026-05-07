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

export interface UploadResult {
  path: string
  name: string
  size: number
}

export interface ToolInfo {
  name: string
  description: string
  input_schema: Record<string, unknown>
}

export interface ToolTestResult {
  output: string
  error: string
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
  tool_call_stats?: ToolCallStats | null
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

export interface Subtask {
  id: number
  session_id: number
  task_id: string
  parent_id: string
  agent_name: string
  agent_type: string
  challenge_type: string
  title: string
  description: string
  target: string
  status: string
  result: string
  error: string
  sort_order: number
  created_at: string
  updated_at: string
  completed_at: string | null
}

export interface ToolCall {
  id: number
  session_id: number
  subtask_id?: number
  task_id: string
  agent_name: string
  agent_type: string
  tool_use_id: string
  tool_name: string
  input: string
  output: string
  status: string
  error: string
  started_at: string
  completed_at: string | null
  duration_ms: number
}

export interface ToolCallGroupStat {
  name: string
  total_calls: number
  success_calls: number
  failed_calls: number
  total_duration_ms: number
  avg_duration_ms: number
  last_used: string
}

export interface ToolCallStats {
  total_calls: number
  success_calls: number
  failed_calls: number
  total_duration_ms: number
  avg_duration_ms: number
  by_tool: ToolCallGroupStat[]
  by_agent: ToolCallGroupStat[]
}

export interface FlowTemplate {
  id: number
  challenge_type: string
  title: string
  description: string
  content: string
  tags: string
  created_at: string
  updated_at: string
}

export type PlanOperationType = 'add' | 'remove' | 'modify' | 'reorder'

export interface PlanOperation {
  op: PlanOperationType
  id?: number
  after_id?: number
  title?: string
  description?: string
}

export interface PlanPatch {
  message?: string
  operations: PlanOperation[]
}

export interface PlanSuggestion {
  patch: PlanPatch
  source: string
  error?: string
}

export interface PlanSuggestResult {
  suggestion: PlanSuggestion
  applied: boolean
  plan?: Subtask[]
}

export interface GeneratePlanResult {
  created: number
  source: string
  plan: Subtask[]
}

export interface SolveRequest {
  challenge_type: string
  description: string
  target: string
  files: string[]
  plan_with_llm?: boolean
  template_id?: number
}

export interface SolveResponse {
  session_id: number
  status: string
  planned_subtasks?: number
  plan_source?: string
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

  exportSession: (id: number, format: 'json' | 'markdown' = 'markdown') => {
    const url = `${BASE}/api/sessions/${id}/export?format=${format}`
    const a = document.createElement('a')
    a.href = url
    a.download = `session-${id}.${format === 'markdown' ? 'md' : 'json'}`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  },

  solve: (data: SolveRequest) =>
    request<SolveResponse>('/api/solve', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getSessionPlan: (id: number, status?: string) => {
    const params = new URLSearchParams()
    if (status) params.set('status', status)
    const suffix = params.toString() ? `?${params}` : ''
    return request<Subtask[]>(`/api/sessions/${id}/plan${suffix}`)
  },

  generatePlan: (id: number, opts?: { llm?: boolean; replace?: boolean; template_id?: number }) => {
    const params = new URLSearchParams()
    if (opts?.llm) params.set('llm', 'true')
    if (opts?.replace) params.set('replace', 'true')
    if (opts?.template_id) params.set('template_id', String(opts.template_id))
    const suffix = params.toString() ? `?${params}` : ''
    return request<GeneratePlanResult>(`/api/sessions/${id}/plan/generate${suffix}`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },

  patchPlan: (id: number, patch: PlanPatch) =>
    request<Subtask[]>(`/api/sessions/${id}/plan/patch`, {
      method: 'POST',
      body: JSON.stringify(patch),
    }),

  refinePlan: (id: number) =>
    request<Subtask[]>(`/api/sessions/${id}/plan/refine`, { method: 'POST' }),

  suggestPlanPatch: (id: number, opts?: { llm?: boolean; apply?: boolean }) => {
    const params = new URLSearchParams()
    if (opts?.llm) params.set('llm', 'true')
    if (opts?.apply) params.set('apply', 'true')
    const suffix = params.toString() ? `?${params}` : ''
    return request<PlanSuggestResult>(`/api/sessions/${id}/plan/suggest-patch${suffix}`, {
      method: 'POST',
    })
  },

  getSessionSubtasks: (id: number, status?: string) => {
    const params = new URLSearchParams()
    if (status) params.set('status', status)
    const suffix = params.toString() ? `?${params}` : ''
    return request<Subtask[]>(`/api/sessions/${id}/subtasks${suffix}`)
  },

  getSessionToolCalls: (id: number, limit = 200) =>
    request<ToolCall[]>(`/api/sessions/${id}/tool-calls?limit=${limit}`),

  getSessionToolCallStats: (id: number) =>
    request<ToolCallStats>(`/api/sessions/${id}/tool-calls/stats`),

  getToolCallStats: () => request<ToolCallStats>('/api/tool-calls/stats'),

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

  uploadFile: async (file: File, sessionId?: string): Promise<UploadResult> => {
    const formData = new FormData()
    formData.append('file', file)
    if (sessionId) formData.append('session_id', sessionId)

    const res = await fetch(`${BASE}/api/upload`, {
      method: 'POST',
      body: formData,
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `上传失败: ${res.status}`)
    }
    return res.json()
  },

  getTools: () => request<ToolInfo[]>('/api/tools'),

  testTool: (name: string, input: Record<string, unknown>) =>
    request<ToolTestResult>(`/api/tools/${name}/test`, {
      method: 'POST',
      body: JSON.stringify({ input }),
    }),

  getTemplates: (type?: string) => {
    const params = new URLSearchParams()
    if (type) params.set('type', type)
    const suffix = params.toString() ? `?${params}` : ''
    return request<FlowTemplate[]>(`/api/templates${suffix}`)
  },

  getTemplate: (id: number) => request<FlowTemplate>(`/api/templates/${id}`),

  createTemplate: (data: Omit<FlowTemplate, 'id' | 'created_at' | 'updated_at'>) =>
    request<FlowTemplate>('/api/templates', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateTemplate: (id: number, data: Omit<FlowTemplate, 'id' | 'created_at' | 'updated_at'>) =>
    request<FlowTemplate>(`/api/templates/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  deleteTemplate: (id: number) =>
    request<{ ok: boolean }>(`/api/templates/${id}`, { method: 'DELETE' }),
}
