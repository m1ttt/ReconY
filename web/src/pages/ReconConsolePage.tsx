import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { api, type TargetFilter } from '../api/client'
import { useWebSocket } from '../hooks/useWebSocket'
import { useStore } from '../store'
import { useReconSession, type ReconStep } from '../store/reconSession'
import { SelectableDataTable } from '../components/SelectableDataTable'
import { GroupedURLTable } from '../components/GroupedURLTable'
import { ToolResultView } from '../components/ToolResultView'
import { ActionPanel } from '../components/ActionPanel'
import { SessionTimeline } from '../components/SessionTimeline'
import { AuthModal } from '../components/AuthModal'
import { Crosshair, Zap } from 'lucide-react'

// Column definitions per result type
// Default sort column per result type (alphabetical grouping)
const defaultSortMap: Record<string, string> = {
  subdomains: 'hostname', urls: 'url', ports: 'ip_address', parameters: 'url',
  technologies: 'name', vulnerabilities: 'severity', secrets: 'secret_type',
  screenshots: 'url', cloud_assets: 'provider', historical_urls: 'url',
  classifications: 'url', dns: 'record_type', whois: 'domain',
}

const columnDefs: Record<string, { columns: any[]; getId: (row: any) => string }> = {
  subdomains: {
    getId: (r) => r.id,
    columns: [
      { key: 'hostname', label: 'Hostname', mono: true },
      {
        key: 'ip_addresses', label: 'IPs', mono: true,
        render: (r: any) => {
          try { return JSON.parse(r.ip_addresses || '[]').join(', ') } catch { return r.ip_addresses || '—' }
        },
      },
      {
        key: 'is_alive', label: 'Alive',
        render: (r: any) => (
          <span className={r.is_alive ? 'text-completed' : 'text-muted'}>
            {r.is_alive ? 'alive' : 'unknown'}
          </span>
        ),
      },
      { key: 'source', label: 'Source', mono: true, className: 'text-muted' },
    ],
  },
  ports: {
    getId: (r) => r.id,
    columns: [
      { key: 'ip_address', label: 'IP', mono: true },
      {
        key: 'port', label: 'Port', mono: true,
        render: (r: any) => <span className="text-accent">{r.port}</span>,
      },
      { key: 'protocol', label: 'Proto', mono: true },
      { key: 'service', label: 'Service', mono: true },
      { key: 'version', label: 'Version', mono: true, className: 'text-muted' },
    ],
  },
  urls: {
    getId: (r) => r.id,
    columns: [
      { key: 'url', label: 'URL', mono: true },
      {
        key: 'status_code', label: 'Status', mono: true,
        render: (r: any) => {
          const code = r.status_code
          const color = code >= 200 && code < 300 ? 'text-completed' : code >= 300 && code < 400 ? 'text-medium' : code >= 400 ? 'text-failed' : 'text-muted'
          return <span className={color}>{code || '—'}</span>
        },
      },
      { key: 'source', label: 'Source', mono: true, className: 'text-muted' },
    ],
  },
  technologies: {
    getId: (r) => r.id,
    columns: [
      { key: 'name', label: 'Technology', mono: true },
      { key: 'version', label: 'Version', mono: true },
      { key: 'category', label: 'Category', mono: true, className: 'text-muted' },
      { key: 'url', label: 'URL', mono: true, className: 'text-muted' },
    ],
  },
  vulnerabilities: {
    getId: (r) => r.id,
    columns: [
      {
        key: 'severity', label: 'Severity',
        render: (r: any) => {
          const colors: Record<string, string> = { critical: 'text-critical', high: 'text-high', medium: 'text-medium', low: 'text-low', info: 'text-info' }
          return <span className={`font-mono text-xs uppercase ${colors[r.severity] || 'text-muted'}`}>{r.severity}</span>
        },
      },
      { key: 'name', label: 'Name', mono: true },
      { key: 'url', label: 'URL', mono: true, className: 'text-muted' },
    ],
  },
  secrets: {
    getId: (r) => r.id,
    columns: [
      { key: 'secret_type', label: 'Type', mono: true },
      { key: 'value', label: 'Value', mono: true, render: (r: any) => <span className="text-failed bg-failed/10 px-1 rounded">{(r.value || '').slice(0, 40)}</span> },
      { key: 'source', label: 'Source', mono: true, className: 'text-muted' },
    ],
  },
  screenshots: {
    getId: (r) => r.id,
    columns: [
      { key: 'url', label: 'URL', mono: true },
      { key: 'title', label: 'Title' },
      { key: 'status_code', label: 'Status', mono: true },
    ],
  },
  cloud_assets: {
    getId: (r) => r.id,
    columns: [
      { key: 'provider', label: 'Provider', mono: true },
      { key: 'name', label: 'Name', mono: true },
      {
        key: 'is_public', label: 'Public',
        render: (r: any) => <span className={r.is_public ? 'text-critical' : 'text-muted'}>{r.is_public ? 'PUBLIC' : 'private'}</span>,
      },
    ],
  },
  historical_urls: {
    getId: (r) => r.id,
    columns: [
      { key: 'url', label: 'URL', mono: true },
      { key: 'source', label: 'Source', mono: true, className: 'text-muted' },
    ],
  },
  parameters: {
    getId: (r) => r.id,
    columns: [
      { key: 'url', label: 'URL', mono: true },
      { key: 'name', label: 'Param', mono: true, render: (r: any) => <span className="text-accent">{r.name}</span> },
      { key: 'param_type', label: 'Type', mono: true },
    ],
  },
  classifications: {
    getId: (r) => r.id,
    columns: [
      { key: 'url', label: 'URL', mono: true },
      { key: 'site_type', label: 'Type', mono: true },
      { key: 'waf_detected', label: 'WAF', mono: true },
      { key: 'ssl_grade', label: 'SSL', mono: true },
    ],
  },
  dns: {
    getId: (r) => r.id,
    columns: [
      { key: 'host', label: 'Host', mono: true },
      { key: 'record_type', label: 'Type', mono: true },
      { key: 'value', label: 'Value', mono: true },
    ],
  },
  whois: {
    getId: (r) => r.id,
    columns: [
      { key: 'domain', label: 'Domain', mono: true },
      { key: 'registrar', label: 'Registrar', mono: true },
      { key: 'org', label: 'Org', mono: true },
    ],
  },
}

// Map result types to API fetch functions. Query string includes sort params.
const fetchMap: Record<string, (wsId: string, qs: string) => Promise<any>> = {
  subdomains: (id, qs) => api.listSubdomains(id, qs),
  ports: (id, qs) => api.listPorts(id, qs),
  urls: (id, qs) => api.listDiscoveredURLs(id, qs),
  technologies: (id, qs) => api.listTechnologies(id, qs),
  vulnerabilities: (id, qs) => api.listVulnerabilities(id, qs),
  secrets: (id, qs) => api.listSecrets(id, qs),
  screenshots: (id, qs) => api.listScreenshots(id, qs),
  cloud_assets: (id, qs) => api.listCloudAssets(id, qs),
  historical_urls: (id, qs) => api.listHistoricalURLs(id, qs),
  parameters: (id, qs) => api.listParameters(id, qs),
  classifications: (id, qs) => api.listClassifications(id, qs),
  dns: (id, qs) => api.listDNS(id, qs),
  whois: (id, qs) => api.listWhois(id, qs),
}

const MAX_RESULTS_PER_PAGE = 500

function buildQueryString(
  type: string,
  sort?: string,
  order?: string,
  scanJobId?: string,
  page = 1,
  perPage = MAX_RESULTS_PER_PAGE
): string {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })
  const sortCol = sort || defaultSortMap[type]
  if (sortCol) {
    params.set('sort', sortCol)
    params.set('order', order || 'ASC')
  }
  if (scanJobId) {
    params.set('scan_job_id', scanJobId)
  }
  return '?' + params.toString()
}

async function fetchAllResultPages(
  workspaceId: string,
  type: string,
  sort?: string,
  order?: string,
  scanJobId?: string
): Promise<any[]> {
  const fetcher = fetchMap[type]
  if (!fetcher) return []

  const allRows: any[] = []
  let page = 1

  for (;;) {
    const qs = buildQueryString(type, sort, order, scanJobId, page, MAX_RESULTS_PER_PAGE)
    const res = await fetcher(workspaceId, qs)

    if (Array.isArray(res)) {
      return res
    }

    const rows = Array.isArray(res?.data) ? res.data : []
    allRows.push(...rows)

    const total = typeof res?.total === 'number' ? res.total : null
    if ((total != null && allRows.length >= total) || rows.length < MAX_RESULTS_PER_PAGE) {
      break
    }
    page++
  }

  return allRows
}

// Map tool to the result types it produces (first = primary, shown by default)
const toolResultMap: Record<string, string[]> = {
  subfinder: ['subdomains'], crtsh: ['subdomains'], amass: ['subdomains'], puredns: ['subdomains'],
  nmap: ['ports'], shodan: ['ports'], censys: ['ports'],
  httpx: ['technologies'], waf_detect: ['classifications'], ssl_analyze: ['classifications'], classify: ['classifications'],
  katana: ['urls'], ffuf: ['urls'], feroxbuster: ['urls'], gowitness: ['screenshots'], cmseek: ['technologies'],
  paramspider: ['parameters'], jsluice: ['urls', 'secrets'], secretfinder: ['secrets'],
  'static-analysis': ['urls', 'secrets'],
  bucket_enum: ['cloud_assets'], gitdork: ['urls'], js_secrets: ['secrets'],
  nuclei: ['vulnerabilities'],
  whois: ['whois'], dns: ['dns'], waybackurls: ['historical_urls'], gau: ['historical_urls'],
}

function primaryResultType(toolName: string): string {
  return (toolResultMap[toolName] || ['subdomains'])[0]
}

function deriveSessionState(steps: ReconStep[]): 'idle' | 'running' | 'reviewing' {
  if (steps.some((s) => s.status === 'running')) return 'running'
  if (steps.some((s) => s.status === 'completed' || s.status === 'failed')) return 'reviewing'
  return 'idle'
}

export function ReconConsolePage() {
  const { workspaceId } = useParams()
  const events = useStore((s) => s.events)
  const {
    state, steps, currentStepIndex, selectedIds, selectedDataType,
    setState, addStep, loadSteps, updateStep, setCurrentStep,
    toggleSelected, selectAll, clearSelection,
    setSelectedDataType, reset,
  } = useReconSession()

  const [resultData, setResultData] = useState<any[]>([])
  const [loadingResults, setLoadingResults] = useState(false)
  const [showAuthModal, setShowAuthModal] = useState(false)
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const [scanLogs, setScanLogs] = useState<Array<{ stream: string; line: string; timestamp?: string }>>([])
  const prevWorkspaceRef = useRef(workspaceId)

  const loadResults = useCallback(async (type: string, sort?: string, order?: 'asc' | 'desc', scanJobId?: string) => {
    if (!workspaceId || !fetchMap[type]) return
    setLoadingResults(true)
    try {
      const data = await fetchAllResultPages(workspaceId, type, sort, (order || 'asc').toUpperCase(), scanJobId)
      setResultData(data)
      setSelectedDataType(type)
      if (sort) { setSortKey(sort); setSortDir(order || 'asc') }
      else { setSortKey(defaultSortMap[type] || null); setSortDir('asc') }
      clearSelection()
    } catch (e) {
      console.error('Failed to load results:', e)
      setResultData([])
    }
    setLoadingResults(false)
  }, [workspaceId, clearSelection, setSelectedDataType])

  const loadScanLogs = useCallback(async (scanJobId?: string) => {
    if (!workspaceId || !scanJobId) {
      setScanLogs([])
      return
    }
    try {
      const logs = await api.getScanLogs(workspaceId, scanJobId)
      setScanLogs(Array.isArray(logs) ? logs : [])
    } catch {
      setScanLogs([])
    }
  }, [workspaceId])

  // Reset when switching workspaces
  useEffect(() => {
    if (prevWorkspaceRef.current !== workspaceId) {
      prevWorkspaceRef.current = workspaceId
      reset()
      setResultData([])
      setScanLogs([])
    }
  }, [workspaceId])

  // Reconstruct session timeline from backend (source of truth)
  useEffect(() => {
    if (!workspaceId) return
    let cancelled = false

    api.listScans(workspaceId).then((jobs: any[]) => {
      if (cancelled) return
      if (!jobs || jobs.length === 0) {
        reset()
        setResultData([])
        return
      }
      // Sort oldest first to build timeline in order
      const sorted = [...jobs].sort((a, b) => a.created_at.localeCompare(b.created_at))
      const rebuilt: ReconStep[] = sorted.map((j) => ({
        id: j.id,
        toolName: j.tool_name,
        phaseName: `Phase ${j.phase}`,
        status: j.status === 'completed' ? 'completed' : j.status === 'running' ? 'running' : 'failed',
        resultCount: j.result_count || 0,
        resultType: primaryResultType(j.tool_name),
        timestamp: j.started_at || j.created_at,
        scanJobId: j.id,
      }))
      // Replace all steps (backend is source of truth)
      loadSteps(rebuilt)
      setState(deriveSessionState(rebuilt))
      // Load results for the last completed step
      const lastCompleted = [...rebuilt].reverse().find((s) => s.status === 'completed')
      if (lastCompleted) {
        setLoadingResults(true)
        fetchAllResultPages(
          workspaceId,
          lastCompleted.resultType,
          defaultSortMap[lastCompleted.resultType],
          'ASC',
          lastCompleted.scanJobId
        ).then((rows) => {
          if (cancelled) return
          setResultData(rows)
          setSelectedDataType(lastCompleted.resultType)
          setSortKey(defaultSortMap[lastCompleted.resultType] || null)
          setSortDir('asc')
        }).catch(() => {}).finally(() => setLoadingResults(false))
      }
    }).catch(() => {})

    return () => { cancelled = true }
  }, [workspaceId])

  useWebSocket(workspaceId)

  // Load logs for selected step and poll while running.
  useEffect(() => {
    const selectedStep = currentStepIndex >= 0 ? steps[currentStepIndex] : null
    if (!workspaceId || !selectedStep?.scanJobId) {
      setScanLogs([])
      return
    }

    let cancelled = false
    const fetchLogs = async () => {
      try {
        const logs = await api.getScanLogs(workspaceId, selectedStep.scanJobId!)
        if (cancelled) return
        setScanLogs(Array.isArray(logs) ? logs : [])
      } catch {
        if (!cancelled) setScanLogs([])
      }
    }

    fetchLogs()

    if (selectedStep.status !== 'running') {
      return () => { cancelled = true }
    }

    const timer = setInterval(fetchLogs, 5000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [workspaceId, currentStepIndex, steps])

  // Fallback live stream: poll result rows while the selected step is running.
  // This keeps UI updating even when WS result events are unavailable.
  useEffect(() => {
    const selectedStep = currentStepIndex >= 0 ? steps[currentStepIndex] : null
    if (!workspaceId || !selectedStep?.scanJobId || selectedStep.status !== 'running') return

    let cancelled = false
    const pollResults = async () => {
      const type = selectedDataType || selectedStep.resultType
      if (!type || !fetchMap[type]) return
      try {
        const data = await fetchAllResultPages(
          workspaceId,
          type,
          sortKey || defaultSortMap[type],
          sortDir.toUpperCase(),
          selectedStep.scanJobId
        )
        if (!cancelled) {
          setResultData(data)
          setSelectedDataType(type)
        }
      } catch {
        // Keep previous data on transient poll errors.
      }
    }

    pollResults()
    const timer = setInterval(pollResults, 5000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [workspaceId, currentStepIndex, steps, selectedDataType, sortKey, sortDir, setSelectedDataType])

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        clearSelection()
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'a' && resultData.length > 0) {
        e.preventDefault()
        const def = columnDefs[selectedDataType || 'subdomains']
        if (def) selectAll(resultData.map(def.getId))
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [resultData, selectedDataType])

  // Watch for scan events to update timeline
  useEffect(() => {
    const lastEvent = events[events.length - 1]
    if (!lastEvent) return

    if (lastEvent.type === 'scan.log_line' && lastEvent.scan_job_id) {
      const selectedStep = currentStepIndex >= 0 ? steps[currentStepIndex] : null
      if (selectedStep?.scanJobId === lastEvent.scan_job_id) {
        const data = (lastEvent.data || {}) as any
        if (typeof data.line === 'string' && data.line.length > 0) {
          setScanLogs((prev) => {
            const next = [...prev, {
              stream: data.stream || 'stdout',
              line: data.line,
              timestamp: String(lastEvent.timestamp || ''),
            }]
            return next.length > 600 ? next.slice(next.length - 600) : next
          })
        }
      }
    }

    if (lastEvent.type === 'result.new_url' && lastEvent.scan_job_id) {
      const runningStep = steps.find((s) => s.status === 'running' && s.scanJobId === lastEvent.scan_job_id)
      if (runningStep && (runningStep.resultType === 'urls' || runningStep.resultType === 'historical_urls')) {
        const incoming = lastEvent.data as any
        if (incoming && typeof incoming.url === 'string') {
          setSelectedDataType(runningStep.resultType)
          setResultData((prev) => {
            const exists = prev.some((row: any) =>
              (incoming.id && row.id === incoming.id) || row.url === incoming.url
            )
            if (exists) return prev
            return [...prev, incoming]
          })
        }
      }
    }

    // Capture the real scan_job_id from engine's scan.started event
    if (lastEvent.type === 'scan.started' && lastEvent.tool_name && lastEvent.scan_job_id) {
      const currentStep = steps.find((s) => s.status === 'running' && s.scanJobId === lastEvent.scan_job_id)
        || steps.find((s) => s.status === 'running' && s.toolName === lastEvent.tool_name && !s.scanJobId)
      if (currentStep && currentStep.scanJobId !== lastEvent.scan_job_id) {
        updateStep(currentStep.id, { scanJobId: lastEvent.scan_job_id })
        loadScanLogs(lastEvent.scan_job_id)
      }
    }

    if (lastEvent.type === 'scan.completed' && lastEvent.tool_name) {
      const currentStep = (lastEvent.scan_job_id
        ? steps.find((s) => s.status === 'running' && s.scanJobId === lastEvent.scan_job_id)
        : undefined) || steps.find((s) => s.status === 'running' && s.toolName === lastEvent.tool_name)
      if (currentStep) {
        const scanJobId = lastEvent.scan_job_id || currentStep.scanJobId
        updateStep(currentStep.id, {
          status: 'completed',
          resultCount: lastEvent.data?.result_count || 0,
          scanJobId,
        })
        const remaining = steps.filter((s) => s.status === 'running' && s.id !== currentStep.id)
        setState(remaining.length > 0 ? 'running' : 'reviewing')
        loadResults(primaryResultType(lastEvent.tool_name), undefined, undefined, scanJobId)
        loadScanLogs(scanJobId)
      }
    }

    if (lastEvent.type === 'scan.failed' && lastEvent.tool_name) {
      const currentStep = (lastEvent.scan_job_id
        ? steps.find((s) => s.status === 'running' && s.scanJobId === lastEvent.scan_job_id)
        : undefined) || steps.find((s) => s.status === 'running' && s.toolName === lastEvent.tool_name)
      if (currentStep) {
        const scanJobId = lastEvent.scan_job_id || currentStep.scanJobId
        updateStep(currentStep.id, {
          status: 'failed',
          resultCount: lastEvent.data?.result_count || 0,
          scanJobId,
        })
        const remaining = steps.filter((s) => s.status === 'running' && s.id !== currentStep.id)
        setState(remaining.length > 0 ? 'running' : 'reviewing')
        loadResults(primaryResultType(lastEvent.tool_name), undefined, undefined, scanJobId)
        loadScanLogs(scanJobId)
      }
    }
  }, [events])

  // Load results when clicking a timeline step — show all results of this type
  const handleSelectStep = useCallback((index: number) => {
    setCurrentStep(index)
    const step = steps[index]
    if (step) {
      loadResults(step.resultType, undefined, undefined, step.scanJobId)
      loadScanLogs(step.scanJobId)
    }
  }, [steps, loadResults, loadScanLogs])

  const handleRunTool = useCallback(async (toolName: string) => {
    if (!workspaceId) return
    // Allow parallel runs, but avoid launching the same tool twice at once.
    if (steps.some((s) => s.status === 'running' && s.toolName === toolName)) return

    const resultType = primaryResultType(toolName)

    // Build targets if we have selections
    let targets: TargetFilter | undefined
    if (selectedIds.size > 0 && selectedDataType) {
      targets = {}
      if (selectedDataType === 'subdomains') targets.subdomain_ids = [...selectedIds]
      else if (selectedDataType === 'ports') targets.port_ids = [...selectedIds]
      else if (selectedDataType === 'urls') targets.url_ids = [...selectedIds]
    }

    const stepId = `step-${Date.now()}`
    const step: ReconStep = {
      id: stepId,
      toolName,
      phaseName: '',
      status: 'running',
      resultCount: 0,
      resultType,
      timestamp: new Date().toISOString(),
      targetCount: selectedIds.size || undefined,
    }

    addStep(step)
    setState('running')
    if (!steps.some((s) => s.status === 'running')) {
      setResultData([])
      setScanLogs([])
    }

    try {
      await api.startScan(workspaceId, {
        tool: toolName,
        ...(targets ? { targets } : {}),
      })
      // scanJobId will be captured from the WebSocket scan.started event
    } catch {
      updateStep(stepId, { status: 'failed' })
      setState('reviewing')
    }
  }, [workspaceId, selectedIds, selectedDataType, steps, addStep, setState, updateStep])

  const colDef = columnDefs[selectedDataType || 'subdomains'] || columnDefs.subdomains
  const selectedStep = currentStepIndex >= 0 ? steps[currentStepIndex] : null
  const activeToolName = selectedStep?.toolName || null
  const runningCount = steps.filter((s) => s.status === 'running').length

  return (
    <div className="animate-fade-in h-[calc(100vh-2rem)] flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-3 mb-4 shrink-0">
        <div className="w-8 h-8 rounded-lg bg-accent/10 border border-accent/20 flex items-center justify-center">
          <Crosshair size={16} className="text-accent" />
        </div>
        <div>
          <h1 className="text-lg font-bold text-heading tracking-tight">Interactive Recon</h1>
          <p className="text-[10px] font-mono text-muted">
            {state === 'idle' && 'Choose a tool to start'}
            {state === 'running' && (
              <span className="text-running">
                <Zap size={10} className="inline animate-pulse mr-1" />
                Running {runningCount} scan{runningCount === 1 ? '' : 's'}...
              </span>
            )}
            {state === 'reviewing' && `${resultData.length} results — select targets for next step`}
          </p>
        </div>
      </div>

      {/* 3-Panel Layout */}
      <div className="flex-1 flex gap-3 min-h-0">
        {/* Left: Timeline */}
        <div className="w-44 shrink-0 bg-surface border border-border rounded-lg overflow-hidden">
          <SessionTimeline
            steps={steps}
            currentStepIndex={currentStepIndex}
            onSelectStep={handleSelectStep}
          />
        </div>

        {/* Center: Results */}
        <div className="flex-1 min-w-0 flex flex-col">
          {/* Result type pills for multi-output tools */}
          {(() => {
            const types = selectedStep ? (toolResultMap[selectedStep.toolName] || ['subdomains']) : []
            if (types.length <= 1) return null
            return (
              <div className="flex gap-1 mb-2 px-1">
                {types.map((t) => (
                  <button
                    key={t}
                    onClick={() => loadResults(t, undefined, undefined, selectedStep?.scanJobId)}
                    className={`px-2.5 py-1 rounded text-[10px] font-mono transition-all border ${
                      selectedDataType === t
                        ? 'bg-accent/15 border-accent/30 text-accent'
                        : 'bg-surface border-border text-muted hover:text-text hover:border-border'
                    }`}
                  >
                    {t}
                  </button>
                ))}
              </div>
            )
          })()}
          {state === 'idle' && resultData.length === 0 ? (
            <div className="flex-1 flex items-center justify-center bg-surface border border-dashed border-border rounded-lg">
              <div className="text-center">
                <Crosshair size={40} className="mx-auto text-muted/15 mb-4" />
                <p className="text-sm font-mono text-muted">No results yet</p>
                <p className="text-xs text-muted/50 mt-1">Pick a tool from the right panel to begin</p>
              </div>
            </div>
          ) : state === 'running' && resultData.length === 0 ? (
            <div className="flex-1 flex items-center justify-center bg-surface border border-border rounded-lg">
              <div className="text-center">
                <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto mb-4" />
                <p className="text-sm font-mono text-accent">
                  Running {runningCount} scan{runningCount === 1 ? '' : 's'}...
                </p>
                <p className="text-xs text-muted/50 mt-1">Results stream live as they are discovered</p>
              </div>
            </div>
          ) : (() => {
            // Tool-specific views for tools that need custom rendering
            const toolsWithCustomView = ['waf_detect', 'ssl_analyze', 'classify', 'whois', 'dns']
            if (activeToolName && toolsWithCustomView.includes(activeToolName)) {
              return (
                <ToolResultView
                  toolName={activeToolName}
                  data={resultData}
                  selectedIds={selectedIds}
                  onToggle={toggleSelected}
                  onSelectAll={(ids) => selectAll(ids)}
                />
              )
            }
            if (selectedDataType === 'urls' || selectedDataType === 'historical_urls') {
              return (
                <GroupedURLTable
                  data={resultData}
                  loading={loadingResults}
                  selectedIds={selectedIds}
                  onToggle={toggleSelected}
                  onSelectAll={(ids) => selectAll(ids)}
                  onClearSelection={clearSelection}
                />
              )
            }
            return (
              <SelectableDataTable
                columns={colDef.columns}
                data={resultData}
                loading={loadingResults}
                emptyMessage="No results found"
                selectedIds={selectedIds}
                onToggle={toggleSelected}
                onSelectAll={(ids) => selectAll(ids)}
                onClearSelection={clearSelection}
                getId={colDef.getId}
                sortKey={sortKey}
                sortDir={sortDir}
                onSort={(key, dir) => {
                  const step = currentStepIndex >= 0 ? steps[currentStepIndex] : null
                  if (selectedDataType) loadResults(selectedDataType, key, dir, step?.scanJobId)
                }}
              />
            )
          })()}

          {(scanLogs.length > 0 || state === 'running') && (
            <div className="mt-2 bg-deep border border-border rounded-lg overflow-hidden">
              <div className="px-3 py-1.5 border-b border-border bg-raised/40 flex items-center justify-between">
                <span className="text-[10px] font-mono uppercase tracking-wider text-muted">Logs</span>
                <span className="text-[10px] font-mono text-muted/70">{scanLogs.length} lines</span>
              </div>
              <div className="max-h-36 overflow-y-auto p-2 space-y-0.5">
                {scanLogs.length === 0 ? (
                  <p className="text-[11px] font-mono text-muted/60">Waiting for log lines...</p>
                ) : (
                  scanLogs.map((l, i) => (
                    <div
                      key={`${l.timestamp || 't'}-${i}`}
                      className={`text-[11px] font-mono leading-snug ${l.stream === 'stderr' ? 'text-failed/80' : 'text-text/75'}`}
                    >
                      <span className="text-muted/40 mr-1">{l.stream === 'stderr' ? 'ERR' : 'OUT'}</span>
                      {l.line}
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        {/* Right: Actions */}
        <div className="w-60 shrink-0 bg-surface border border-border rounded-lg overflow-hidden">
          <ActionPanel
            selectedDataType={selectedDataType}
            selectedCount={selectedIds.size}
            onRunTool={handleRunTool}
            onSetupAuth={() => setShowAuthModal(true)}
            runningCount={runningCount}
          />
        </div>
      </div>

      {/* Auth Modal */}
      {showAuthModal && workspaceId && (
        <AuthModal
          workspaceId={workspaceId}
          onClose={() => setShowAuthModal(false)}
        />
      )}
    </div>
  )
}
