import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'
import { SeverityBadge } from '../components/StatusBadge'

export function SecretsPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    api.listSecrets(workspaceId).then((res) => { setData(res.data); setTotal(res.total); setLoading(false) })
  }, [workspaceId])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Secrets</h1>
        <p className="text-xs font-mono text-muted mt-1">{total} found</p>
      </div>
      <DataTable loading={loading} data={data} emptyMessage="No secrets found yet"
        columns={[
          { key: 'severity', label: 'Severity', render: (r) => <SeverityBadge severity={r.severity} /> },
          { key: 'secret_type', label: 'Type', mono: true, render: (r) => <span className="text-heading">{r.secret_type}</span> },
          { key: 'value', label: 'Value', mono: true, render: (r) => (
            <code className="text-critical text-xs bg-critical/5 px-1.5 py-0.5 rounded">{r.value?.slice(0, 40)}...</code>
          )},
          { key: 'source_url', label: 'Source', mono: true, render: (r) => (
            <span className="text-muted text-xs truncate max-w-xs block">{r.source_url}</span>
          )},
          { key: 'source', label: 'Tool', mono: true },
        ]}
      />
    </div>
  )
}
