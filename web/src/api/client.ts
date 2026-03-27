const BASE = '/api/v1'

export interface AskAIMessage {
  role: 'user' | 'assistant'
  content: string
}

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Workspaces
  listWorkspaces: () => request<any[]>('/workspaces'),
  createWorkspace: (data: { domain: string; name?: string; description?: string }) =>
    request<any>('/workspaces', { method: 'POST', body: JSON.stringify(data) }),
  getWorkspace: (id: string) => request<any>(`/workspaces/${id}`),
  deleteWorkspace: (id: string) => request<void>(`/workspaces/${id}`, { method: 'DELETE' }),
  getWorkspaceStats: (id: string) => request<any>(`/workspaces/${id}/stats`),

  // Scans
  startScan: (wsId: string, data: { workflow?: string; phases?: number[]; tool?: string; targets?: TargetFilter }) =>
    request<any>(`/workspaces/${wsId}/scans`, { method: 'POST', body: JSON.stringify(data) }),
  listScans: (wsId: string) => request<any[]>(`/workspaces/${wsId}/scans`),
  cancelScan: (wsId: string, jobId: string) =>
    request<any>(`/workspaces/${wsId}/scans/${jobId}/cancel`, { method: 'POST' }),
  getScanLogs: (wsId: string, jobId: string) => request<any[]>(`/workspaces/${wsId}/scans/${jobId}/logs`),

  // Results
  listSubdomains: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/subdomains${params}`),
  listPorts: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/ports${params}`),
  listTechnologies: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/technologies${params}`),
  listVulnerabilities: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/vulnerabilities${params}`),
  listDNS: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/dns${params}`),
  listWhois: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/whois${params}`),
  listHistoricalURLs: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/historical-urls${params}`),
  listDiscoveredURLs: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/urls${params}`),
  listParameters: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/parameters${params}`),
  listClassifications: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/classifications${params}`),
  listSecrets: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/secrets${params}`),
  listScreenshots: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/screenshots${params}`),
  listCloudAssets: (wsId: string, params = '') => request<any>(`/workspaces/${wsId}/cloud-assets${params}`),

  // Auth Credentials
  listAuth: (wsId: string) => request<any[]>(`/workspaces/${wsId}/auth`),
  createAuth: (wsId: string, data: any) =>
    request<any>(`/workspaces/${wsId}/auth`, { method: 'POST', body: JSON.stringify(data) }),
  updateAuth: (wsId: string, credId: string, data: any) =>
    request<any>(`/workspaces/${wsId}/auth/${credId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAuth: (wsId: string, credId: string) =>
    request<void>(`/workspaces/${wsId}/auth/${credId}`, { method: 'DELETE' }),
  testAuth: (wsId: string, credId: string) =>
    request<any>(`/workspaces/${wsId}/auth/${credId}/test`, { method: 'POST' }),

  // Workflows
  listWorkflows: () => request<any[]>('/workflows'),
  getWorkflow: (name: string) => request<any>(`/workflows/${name}`),
  duplicateWorkflow: (name: string) =>
    request<any>(`/workflows/${name}/duplicate`, { method: 'POST' }),

  // System
  getConfig: () => request<any>('/config'),
  updateConfig: (data: any) => request<any>('/config', { method: 'PUT', body: JSON.stringify(data) }),
  checkTools: () => request<any[]>('/tools/check'),
  getToolRegistry: () => request<Record<number, ToolRegistryEntry[]>>('/tools/registry'),

  // AI
  askAI: (query: string) => request<{ result: string }>('/ai/ask', { method: 'POST', body: JSON.stringify({ query }) }).then(res => res.result),
  askAIStream: async (
    query: string,
    messages: AskAIMessage[],
    handlers: {
      onStatus?: (status: string) => void
      onChunk?: (chunk: string) => void
      onDone?: () => void
      onError?: (error: string) => void
      onLangGraphEvent?: (event: { event?: string; name?: string; data?: Record<string, unknown> }) => void
    }
  ) => {
    const res = await fetch(`${BASE}/ai/ask/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, messages }),
    })

    if (!res.ok || !res.body) {
      const body = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(body.error || res.statusText)
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const events = buffer.split('\n\n')
      buffer = events.pop() || ''

      for (const rawEvent of events) {
        const dataLine = rawEvent
          .split('\n')
          .find((line) => line.startsWith('data: '))

        if (!dataLine) continue

        try {
          const payload = JSON.parse(dataLine.slice(6))
          if (payload.type === 'status' && payload.status) handlers.onStatus?.(payload.status)
          if (payload.type === 'chunk' && payload.content) handlers.onChunk?.(payload.content)
          if (payload.type === 'done') handlers.onDone?.()
          if (payload.type === 'error' && payload.error) handlers.onError?.(payload.error)
          if (payload.type === 'langgraph') handlers.onLangGraphEvent?.(payload)
        } catch {
          // Ignore malformed SSE payloads from partial chunks.
        }
      }
    }
  },

  // IP Info
  getIpInfo: () => request<{
    ip: string
    country: string
    country_code: string
    city?: string
    is_proxy: boolean
    is_tor: boolean
  }>('/ip-info'),

  // Mullvad status
  getMullvadStatus: () => request<{
    enabled: boolean
    connected: boolean
    status: string
    country?: string
    city?: string
    ip?: string
    hostname?: string
  }>('/mullvad-status'),

  rotateMullvad: (location: string) => request<{
    enabled: boolean
    connected: boolean
    status: string
    country?: string
    city?: string
    ip?: string
  }>('/mullvad-rotate', { method: 'POST', body: JSON.stringify({ location }) }),
}

export interface TargetFilter {
  subdomain_ids?: string[]
  hostnames?: string[]
  port_ids?: string[]
  url_ids?: string[]
}

export interface ToolRegistryEntry {
  name: string
  phase: number
  phase_name: string
  available: boolean
  accepts: string[]
  produces: string[]
}
