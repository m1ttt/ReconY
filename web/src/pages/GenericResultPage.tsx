import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { DataTable } from '../components/DataTable'
import { clsx } from 'clsx'

interface GenericPageProps {
  title: string
  fetchFn: (wsId: string) => Promise<any>
  columns: { key: string; label: string; mono?: boolean; render?: (r: any) => React.ReactNode }[]
  emptyMessage?: string
}

export function GenericResultPage({ title, fetchFn, columns, emptyMessage }: GenericPageProps) {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    fetchFn(workspaceId).then((res) => {
      setData(res.data)
      setTotal(res.total)
      setLoading(false)
    })
  }, [workspaceId])

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">{title}</h1>
        <p className="text-xs font-mono text-muted mt-1">{total} total</p>
      </div>
      <DataTable loading={loading} data={data} columns={columns} emptyMessage={emptyMessage || 'No data yet'} />
    </div>
  )
}

// ===== Discovered URLs =====
export function URLsPage() {
  return (
    <GenericResultPage
      title="Discovered URLs"
      fetchFn={(id) => api.listDiscoveredURLs(id)}
      columns={[
        { key: 'url', label: 'URL', mono: true, render: (r: any) => <span className="text-accent text-xs truncate max-w-md block">{r.url}</span> },
        { key: 'status_code', label: 'Status', mono: true, render: (r: any) => (
          <span className={clsx('font-mono text-xs', r.status_code === 200 ? 'text-completed' : r.status_code >= 400 ? 'text-failed' : 'text-muted')}>
            {r.status_code || '—'}
          </span>
        )},
        { key: 'source', label: 'Source', mono: true },
        { key: 'content_type', label: 'Content-Type', mono: true },
      ]}
      emptyMessage="No URLs discovered yet"
    />
  )
}

// ===== Cloud Assets =====
export function CloudAssetsPage() {
  return (
    <GenericResultPage
      title="Cloud Assets"
      fetchFn={(id) => api.listCloudAssets(id)}
      columns={[
        { key: 'provider', label: 'Provider', mono: true, render: (r: any) => (
          <span className={clsx('text-xs font-mono font-semibold', r.provider === 'aws' ? 'text-high' : r.provider === 'gcp' ? 'text-low' : 'text-medium')}>
            {r.provider?.toUpperCase()}
          </span>
        )},
        { key: 'asset_type', label: 'Type', mono: true },
        { key: 'name', label: 'Name', mono: true, render: (r: any) => <span className="text-heading">{r.name}</span> },
        { key: 'url', label: 'URL', mono: true, render: (r: any) => <span className="text-accent text-xs truncate max-w-xs block">{r.url || '—'}</span> },
        { key: 'is_public', label: 'Public', render: (r: any) => (
          <span className={clsx('text-[11px] font-mono font-bold', r.is_public ? 'text-critical' : 'text-muted')}>
            {r.is_public ? 'PUBLIC' : 'private'}
          </span>
        )},
      ]}
      emptyMessage="No cloud assets found"
    />
  )
}

// ===== DNS Records =====
export function DNSPage() {
  return (
    <GenericResultPage
      title="DNS Records"
      fetchFn={(id) => api.listDNS(id)}
      columns={[
        { key: 'host', label: 'Host', mono: true, render: (r: any) => <span className="text-heading">{r.host}</span> },
        { key: 'record_type', label: 'Type', mono: true, render: (r: any) => (
          <span className="text-accent text-xs font-bold">{r.record_type}</span>
        )},
        { key: 'value', label: 'Value', mono: true, render: (r: any) => <span className="text-text text-xs">{r.value}</span> },
        { key: 'ttl', label: 'TTL', mono: true },
      ]}
      emptyMessage="No DNS records found"
    />
  )
}

// ===== WHOIS =====
export function WhoisPage() {
  return (
    <GenericResultPage
      title="WHOIS Records"
      fetchFn={(id) => api.listWhois(id)}
      columns={[
        { key: 'domain', label: 'Domain', mono: true, render: (r: any) => <span className="text-heading">{r.domain}</span> },
        { key: 'registrar', label: 'Registrar', mono: true },
        { key: 'org', label: 'Organization', mono: true },
        { key: 'country', label: 'Country', mono: true },
        { key: 'asn', label: 'ASN', mono: true, render: (r: any) => <span className="text-accent">{r.asn || '—'}</span> },
        { key: 'asn_org', label: 'ASN Org', mono: true },
        { key: 'creation_date', label: 'Created', mono: true },
        { key: 'expiry_date', label: 'Expires', mono: true },
      ]}
      emptyMessage="No WHOIS data"
    />
  )
}

// ===== Historical URLs =====
export function HistoricalURLsPage() {
  return (
    <GenericResultPage
      title="Historical URLs"
      fetchFn={(id) => api.listHistoricalURLs(id)}
      columns={[
        { key: 'url', label: 'URL', mono: true, render: (r: any) => <span className="text-accent text-xs truncate max-w-lg block">{r.url}</span> },
        { key: 'source', label: 'Source', mono: true, render: (r: any) => (
          <span className="text-muted text-xs">{r.source}</span>
        )},
      ]}
      emptyMessage="No historical URLs found — run waybackurls/gau"
    />
  )
}

// ===== Parameters =====
export function ParametersPage() {
  return (
    <GenericResultPage
      title="Parameters"
      fetchFn={(id) => api.listParameters(id)}
      columns={[
        { key: 'url', label: 'URL', mono: true, render: (r: any) => <span className="text-accent text-xs truncate max-w-sm block">{r.url}</span> },
        { key: 'name', label: 'Param Name', mono: true, render: (r: any) => <span className="text-heading font-semibold">{r.name}</span> },
        { key: 'param_type', label: 'Type', mono: true },
        { key: 'source', label: 'Source', mono: true },
      ]}
      emptyMessage="No parameters discovered — run paramspider"
    />
  )
}

// ===== Classifications =====
export function ClassificationsPage() {
  return (
    <GenericResultPage
      title="Site Classifications"
      fetchFn={(id) => api.listClassifications(id)}
      columns={[
        { key: 'url', label: 'URL', mono: true, render: (r: any) => <span className="text-accent text-xs">{r.url}</span> },
        { key: 'site_type', label: 'Type', render: (r: any) => (
          <span className={clsx('px-2 py-0.5 rounded text-[11px] font-mono font-bold uppercase border',
            r.site_type === 'spa' ? 'bg-low/15 text-low border-low/30' :
            r.site_type === 'ssr' ? 'bg-accent/15 text-accent border-accent/30' :
            r.site_type === 'api' ? 'bg-medium/15 text-medium border-medium/30' :
            'bg-muted/15 text-muted border-muted/30'
          )}>{r.site_type}</span>
        )},
        { key: 'infra_type', label: 'Infra', mono: true },
        { key: 'waf_detected', label: 'WAF', mono: true, render: (r: any) => <span className={r.waf_detected ? 'text-high' : 'text-muted'}>{r.waf_detected || '—'}</span> },
        { key: 'cdn_detected', label: 'CDN', mono: true, render: (r: any) => <span className={r.cdn_detected ? 'text-low' : 'text-muted'}>{r.cdn_detected || '—'}</span> },
        { key: 'ssl_grade', label: 'SSL', mono: true, render: (r: any) => (
          <span className={clsx('font-bold', r.ssl_grade === 'A+' || r.ssl_grade === 'A' ? 'text-completed' : r.ssl_grade === 'F' ? 'text-critical' : 'text-medium')}>
            {r.ssl_grade || '—'}
          </span>
        )},
      ]}
      emptyMessage="No classifications yet — run phase 4"
    />
  )
}

// ===== Screenshots =====
export function ScreenshotsPage() {
  const { workspaceId } = useParams()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!workspaceId) return
    api.listScreenshots(workspaceId).then((res) => { setData(res.data); setLoading(false) })
  }, [workspaceId])

  if (loading) return <div className="text-muted font-mono text-sm p-8">Loading...</div>

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-heading">Screenshots</h1>
        <p className="text-xs font-mono text-muted mt-1">{data.length} captured</p>
      </div>
      {data.length === 0 ? (
        <div className="bg-surface border border-dashed border-border rounded-lg p-12 text-center">
          <p className="text-muted font-mono text-sm">No screenshots captured yet</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-3">
          {data.map((s: any) => (
            <div key={s.id} className="bg-surface border border-border rounded-lg overflow-hidden hover:border-border-bright transition-colors">
              <div className="aspect-video bg-deep flex items-center justify-center">
                <img
                  src={`/api/v1/workspaces/${workspaceId}/screenshots/${s.id}/image`}
                  alt={s.url}
                  className="w-full h-full object-cover"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                />
              </div>
              <div className="p-3">
                <p className="font-mono text-xs text-accent truncate">{s.url}</p>
                {s.title && <p className="text-[10px] text-muted mt-1 truncate">{s.title}</p>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
