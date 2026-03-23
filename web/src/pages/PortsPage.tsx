import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'
import { clsx } from 'clsx'

export function PortsPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    api.listPorts(workspaceId, '?state=open').then((res) => {
      setData(res.data)
      setTotal(res.total)
      setLoading(false)
    })
  }, [workspaceId])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Open Ports</h1>
        <p className="text-xs font-mono text-muted mt-1">{total} open ports</p>
      </div>

      <DataTable
        loading={loading}
        data={data}
        emptyMessage="No ports discovered yet"
        columns={[
          { key: 'ip_address', label: 'IP', mono: true, render: (r) => (
            <span className="text-heading">{r.ip_address}</span>
          )},
          { key: 'port', label: 'Port', mono: true, render: (r) => (
            <span className="text-accent font-semibold">{r.port}</span>
          )},
          { key: 'protocol', label: 'Proto', mono: true },
          { key: 'state', label: 'State', render: (r) => (
            <span className={clsx(
              'text-[11px] font-mono',
              r.state === 'open' ? 'text-completed' : r.state === 'filtered' ? 'text-medium' : 'text-muted'
            )}>
              {r.state}
            </span>
          )},
          { key: 'service', label: 'Service', mono: true, render: (r) => (
            <span className="text-heading">{r.service || '—'}</span>
          )},
          { key: 'version', label: 'Version', mono: true },
        ]}
      />
    </div>
  )
}
