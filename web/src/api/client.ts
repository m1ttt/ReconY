const BASE = '/api/v1'

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
