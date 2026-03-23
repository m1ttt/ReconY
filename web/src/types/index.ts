export interface Workspace {
  id: string
  name: string
  domain: string
  description: string
  config_json: string
  created_at: string
  updated_at: string
  stats?: WorkspaceStats
}

export interface WorkspaceStats {
  subdomains: number
  alive_subdomains: number
  open_ports: number
  technologies: number
  vulnerabilities: number
  secrets: number
  screenshots: number
  cloud_assets: number
}

export interface ScanJob {
  id: string
  workspace_id: string
  phase: number
  tool_name: string
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
  started_at?: string
  finished_at?: string
  result_count: number
  error_message?: string
  created_at: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  per_page: number
}

export interface Subdomain {
  id: string
  hostname: string
  ip_addresses?: string
  is_alive: boolean | number
  source: string
  first_seen: string
  last_seen: string
}

export interface Port {
  id: string
  ip_address: string
  port: number
  protocol: string
  state: string
  service?: string
  version?: string
  banner?: string
}

export interface Vulnerability {
  id: string
  template_id: string
  name: string
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical'
  url: string
  matched_at?: string
  description?: string
  curl_command?: string
}

export interface Technology {
  id: string
  url: string
  name: string
  version?: string
  category?: string
  confidence: number
}

export interface WorkflowInfo {
  id?: string
  name: string
  description: string
  is_builtin: boolean
  phase_ids?: number[]
  config?: string
}

export interface ToolInfo {
  name: string
  available: boolean
  path?: string
  version?: string
  error?: string
}

export type Severity = 'info' | 'low' | 'medium' | 'high' | 'critical'

export const PHASE_NAMES: Record<number, string> = {
  1: 'Passive Recon',
  2: 'Subdomain Enum',
  3: 'Port Scanning',
  4: 'Fingerprinting',
  5: 'Content Discovery',
  6: 'Cloud & Secrets',
  7: 'Vuln Scanning',
}
