import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'

export function TechnologiesPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    api.listTechnologies(workspaceId).then((res) => {
      setData(res.data)
      setTotal(res.total)
      setLoading(false)
    })
  }, [workspaceId])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Technologies</h1>
        <p className="text-xs font-mono text-muted mt-1">{total} detected</p>
      </div>
      <DataTable
        loading={loading}
        data={data}
        emptyMessage="No technologies detected yet"
        columns={[
          { key: 'name', label: 'Technology', render: (r) => <span className="text-heading font-medium">{r.name}</span> },
          { key: 'version', label: 'Version', mono: true },
          { key: 'category', label: 'Category', mono: true, render: (r) => (
            <span className="text-accent/80 text-xs">{r.category || '—'}</span>
          )},
          { key: 'url', label: 'URL', mono: true, render: (r) => (
            <span className="text-muted text-xs truncate max-w-xs block">{r.url}</span>
          )},
          { key: 'confidence', label: 'Confidence', mono: true, render: (r) => (
            <div className="flex items-center gap-2">
              <div className="w-16 h-1.5 bg-deep rounded-full overflow-hidden">
                <div className="h-full bg-accent rounded-full" style={{ width: `${r.confidence}%` }} />
              </div>
              <span className="text-[10px] text-muted">{r.confidence}%</span>
            </div>
          )},
        ]}
      />
    </div>
  )
}
