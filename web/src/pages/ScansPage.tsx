import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { StatusBadge } from '../components/StatusBadge'
import { ChevronDown, ChevronRight } from 'lucide-react'

export function ScansPage() {
  const { workspaceId } = useParams()
  const [scans, setScans] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [expandedJob, setExpandedJob] = useState<string | null>(null)
  const [logs, setLogs] = useState<any[]>([])

  useEffect(() => {
    if (!workspaceId) return
    const load = () => api.listScans(workspaceId).then((data) => { setScans(data); setLoading(false) })
    load()
    const interval = setInterval(load, 3000)
    return () => clearInterval(interval)
  }, [workspaceId])

  const toggleLogs = async (jobId: string) => {
    if (expandedJob === jobId) {
      setExpandedJob(null)
      return
    }
    setExpandedJob(jobId)
    if (!workspaceId) return
    const data = await api.getScanLogs(workspaceId, jobId)
    setLogs(data)
  }

  if (loading) return <div className="text-muted font-mono text-sm p-8">Loading scans...</div>

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Scan Jobs</h1>
        <p className="text-xs font-mono text-muted mt-1">{scans.length} jobs</p>
      </div>

      <div className="space-y-1">
        {scans.map((scan) => (
          <div key={scan.id} className="bg-surface border border-border rounded-lg overflow-hidden">
            <button
              onClick={() => toggleLogs(scan.id)}
              className="w-full flex items-center gap-4 px-4 py-3 hover:bg-raised/30 transition-colors text-left"
            >
              {expandedJob === scan.id ? <ChevronDown size={14} className="text-muted" /> : <ChevronRight size={14} className="text-muted" />}
              <span className="font-mono text-xs text-muted w-8">{scan.phase}</span>
              <span className="font-mono text-xs text-heading w-28">{scan.tool_name}</span>
              <StatusBadge status={scan.status} />
              <span className="font-mono text-xs text-accent ml-auto">{scan.result_count} results</span>
              <span className="font-mono text-[10px] text-muted">{scan.created_at?.slice(11, 19)}</span>
            </button>

            {expandedJob === scan.id && (
              <div className="border-t border-border bg-deep p-3 max-h-80 overflow-y-auto">
                {logs.length === 0 ? (
                  <p className="text-muted font-mono text-xs">No logs</p>
                ) : (
                  <pre className="font-mono text-[11px] leading-relaxed space-y-0">
                    {logs.map((l, i) => (
                      <div key={i} className={l.stream === 'stderr' ? 'text-failed/70' : 'text-text/70'}>
                        <span className="text-muted/40 select-none">{l.stream === 'stderr' ? 'ERR' : 'OUT'} </span>
                        {l.line}
                      </div>
                    ))}
                  </pre>
                )}
              </div>
            )}
          </div>
        ))}

        {scans.length === 0 && (
          <div className="bg-surface border border-dashed border-border rounded-lg p-8 text-center">
            <p className="text-muted font-mono text-sm">No scan jobs yet</p>
          </div>
        )}
      </div>
    </div>
  )
}
