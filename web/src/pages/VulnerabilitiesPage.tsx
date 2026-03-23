import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'
import { SeverityBadge } from '../components/StatusBadge'

export function VulnerabilitiesPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    api.listVulnerabilities(workspaceId).then((res) => {
      setData(res.data)
      setTotal(res.total)
      setLoading(false)
    })
  }, [workspaceId])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Vulnerabilities</h1>
        <p className="text-xs font-mono text-muted mt-1">{total} findings</p>
      </div>

      <DataTable
        loading={loading}
        data={data}
        emptyMessage="No vulnerabilities found yet — run nuclei"
        columns={[
          { key: 'severity', label: 'Severity', render: (r) => <SeverityBadge severity={r.severity} /> },
          { key: 'name', label: 'Name', render: (r) => <span className="text-heading font-medium">{r.name}</span> },
          { key: 'template_id', label: 'Template', mono: true },
          { key: 'url', label: 'URL', mono: true, render: (r) => (
            <span className="text-accent text-xs truncate max-w-xs block">{r.url}</span>
          )},
          { key: 'matched_at', label: 'Match', mono: true, render: (r) => (
            <span className="text-muted text-xs truncate max-w-[200px] block">{r.matched_at || '—'}</span>
          )},
        ]}
      />
    </div>
  )
}
