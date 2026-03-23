import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'
import { clsx } from 'clsx'
import { Waypoints } from 'lucide-react'

export function SubdomainsPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<'all' | 'alive'>('all')
  const navigate = useNavigate()

  useEffect(() => {
    if (!workspaceId) return
    setLoading(true)
    const params = filter === 'alive' ? '?alive=true' : ''
    api.listSubdomains(workspaceId, params).then((res) => {
      setData(res.data)
      setTotal(res.total)
      setLoading(false)
    })
  }, [workspaceId, filter])

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-bold text-heading">Subdomains</h1>
          <p className="text-xs font-mono text-muted mt-1">{total} total</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate(`/workspace/${workspaceId}/recon`)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-mono text-accent/70 border border-accent/20 rounded-md hover:bg-accent/10 hover:text-accent transition-colors"
          >
            <Waypoints size={12} />
            Recon Console
          </button>
          <div className="flex gap-1 bg-deep border border-border rounded-md p-0.5">
            {(['all', 'alive'] as const).map((f) => (
              <button
                key={f}
                onClick={() => setFilter(f)}
                className={clsx(
                  'px-3 py-1 rounded text-xs font-mono transition-colors',
                  filter === f ? 'bg-accent/15 text-accent' : 'text-muted hover:text-text'
                )}
              >
                {f}
              </button>
            ))}
          </div>
        </div>
      </div>

      <DataTable
        loading={loading}
        data={data}
        emptyMessage="No subdomains discovered yet"
        columns={[
          { key: 'hostname', label: 'Hostname', mono: true, render: (r) => (
            <span className="text-heading">{r.hostname}</span>
          )},
          { key: 'ip_addresses', label: 'IPs', mono: true, render: (r) => {
            if (!r.ip_addresses) return <span className="text-muted">—</span>
            try {
              const ips = JSON.parse(r.ip_addresses)
              return <span className="text-subtle">{ips.join(', ')}</span>
            } catch { return <span className="text-subtle">{r.ip_addresses}</span> }
          }},
          { key: 'is_alive', label: 'Status', render: (r) => (
            <span className={clsx(
              'inline-flex items-center gap-1.5 text-[11px] font-mono',
              r.is_alive ? 'text-completed' : 'text-muted'
            )}>
              <span className={clsx('w-1.5 h-1.5 rounded-full', r.is_alive ? 'bg-completed' : 'bg-muted/30')} />
              {r.is_alive ? 'alive' : 'unknown'}
            </span>
          )},
          { key: 'source', label: 'Source', mono: true },
          { key: 'first_seen', label: 'First Seen', mono: true, render: (r) => (
            <span className="text-muted">{r.first_seen?.slice(0, 10)}</span>
          )},
        ]}
      />
    </div>
  )
}
