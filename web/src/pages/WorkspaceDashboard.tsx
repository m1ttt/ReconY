import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { StatCard, StatusBadge } from '../components/StatusBadge'
import { useWebSocket } from '../hooks/useWebSocket'
import { useStore } from '../store'
import { PHASE_NAMES } from '../types'
import type { Workspace, WorkspaceStats, ScanJob } from '../types'
import { Play, Zap, Waypoints } from 'lucide-react'

export function WorkspaceDashboard() {
  const { workspaceId } = useParams()
  const [ws, setWs] = useState<Workspace | null>(null)
  const [stats, setStats] = useState<WorkspaceStats | null>(null)
  const [scans, setScans] = useState<ScanJob[]>([])
  const [selectedWorkflow, setSelectedWorkflow] = useState('full')
  const [workflows, setWorkflows] = useState<any[]>([])
  const toolProgress = useStore((s) => s.toolProgress)
  const [launching, setLaunching] = useState(false)
  const navigate = useNavigate()

  useWebSocket(workspaceId)

  const load = () => {
    if (!workspaceId) return
    api.getWorkspace(workspaceId).then(setWs)
    api.getWorkspaceStats(workspaceId).then(setStats)
    api.listScans(workspaceId).then(setScans)
  }

  useEffect(() => {
    load()
    api.listWorkflows().then(setWorkflows)
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [workspaceId])

  const launchScan = async () => {
    if (!workspaceId) return
    setLaunching(true)
    try {
      await api.startScan(workspaceId, { workflow: selectedWorkflow })
      setTimeout(load, 1000)
    } finally {
      setLaunching(false)
    }
  }

  if (!ws || !stats) {
    return <div className="text-muted font-mono text-sm p-8">Loading workspace...</div>
  }

  const hasRunning = scans.some((s) => s.status === 'running')

  return (
    <div className="animate-fade-in space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-heading tracking-tight">{ws.name}</h1>
          <p className="font-mono text-accent text-sm mt-0.5">{ws.domain}</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate(`/workspace/${workspaceId}/recon`)}
            className="flex items-center gap-2 px-5 py-2 bg-accent text-void font-bold text-sm rounded-md hover:bg-accent-dim transition-colors"
          >
            <Waypoints size={16} />
            Interactive Recon
          </button>
          <div className="flex items-center gap-2">
            <select
              value={selectedWorkflow}
              onChange={(e) => setSelectedWorkflow(e.target.value)}
              className="bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-text focus:outline-none focus:border-accent/50"
            >
              {workflows.filter(w => w.is_builtin).map((wf) => (
                <option key={wf.name} value={wf.name}>{wf.name}</option>
              ))}
            </select>
            <button
              onClick={launchScan}
              disabled={launching}
              className="flex items-center gap-2 px-4 py-2 bg-elevated text-text border border-border font-medium text-sm rounded-md hover:bg-raised hover:border-border-bright transition-colors disabled:opacity-50"
            >
              {hasRunning ? <Zap size={16} className="animate-pulse" /> : <Play size={16} />}
              {launching ? 'Launching...' : hasRunning ? 'Running...' : 'Bulk Scan'}
            </button>
          </div>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 xl:grid-cols-8 gap-3">
        <StatCard label="Subdomains" value={stats.subdomains} accent />
        <StatCard label="Alive" value={stats.alive_subdomains} />
        <StatCard label="Open Ports" value={stats.open_ports} />
        <StatCard label="Technologies" value={stats.technologies} />
        <StatCard label="Vulns" value={stats.vulnerabilities} />
        <StatCard label="Secrets" value={stats.secrets} />
        <StatCard label="Screenshots" value={stats.screenshots} />
        <StatCard label="Cloud Assets" value={stats.cloud_assets} />
      </div>

      {/* Recent Scans */}
      <div>
        <h2 className="text-sm font-mono text-muted uppercase tracking-wider mb-3">Recent Scans</h2>
        {scans.length === 0 ? (
          <div className="bg-surface border border-dashed border-border rounded-lg p-8 text-center">
            <p className="text-muted font-mono text-sm">No scans yet — launch one above</p>
          </div>
        ) : (
          <div className="bg-surface border border-border rounded-lg overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border bg-raised/40">
                  <th className="px-4 py-2 text-left text-[10px] font-mono text-muted uppercase tracking-wider">Phase</th>
                  <th className="px-4 py-2 text-left text-[10px] font-mono text-muted uppercase tracking-wider">Tool</th>
                  <th className="px-4 py-2 text-left text-[10px] font-mono text-muted uppercase tracking-wider">Status</th>
                  <th className="px-4 py-2 text-left text-[10px] font-mono text-muted uppercase tracking-wider">Results</th>
                  <th className="px-4 py-2 text-left text-[10px] font-mono text-muted uppercase tracking-wider">Time</th>
                </tr>
              </thead>
              <tbody>
                {scans.slice(0, 20).map((scan) => (
                  <tr key={scan.id} className="border-b border-border/50 hover:bg-raised/30 transition-colors">
                    <td className="px-4 py-2 text-xs">
                      <span className="font-mono text-muted">{scan.phase}</span>
                      <span className="text-subtle ml-2">{PHASE_NAMES[scan.phase]}</span>
                    </td>
                    <td className="px-4 py-2 font-mono text-xs text-heading">{scan.tool_name}</td>
                    <td className="px-4 py-2"><StatusBadge status={scan.status} /></td>
                    <td className="px-4 py-2 font-mono text-xs">
                      {scan.status === 'running' && toolProgress[scan.tool_name] != null ? (
                        <span className="text-running animate-pulse">{toolProgress[scan.tool_name]}...</span>
                      ) : (
                        <span className={scan.result_count > 0 ? 'text-accent' : 'text-muted'}>{scan.result_count}</span>
                      )}
                    </td>
                    <td className="px-4 py-2 font-mono text-[10px] text-muted">{scan.created_at?.slice(11, 19)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
