import { clsx } from 'clsx'
import { ShieldAlert, ShieldCheck, Globe, Lock, Unlock, Server, Wifi, Eye } from 'lucide-react'

interface Props {
  toolName: string | null
  data: any[]
  selectedIds: Set<string>
  onToggle: (id: string) => void
}

// Tool-specific result renderers.
// Returns null if no custom view exists (falls through to default table).
export function ToolResultView({ toolName, data, selectedIds, onToggle }: Props) {
  if (!toolName) return null
  if (!data.length) {
    return (
      <div className="flex-1 flex items-center justify-center bg-surface border border-border rounded-lg">
        <p className="text-sm font-mono text-muted">No results for this tool</p>
      </div>
    )
  }

  switch (toolName) {
    case 'waf_detect':
      return <WAFResultView data={data} selectedIds={selectedIds} onToggle={onToggle} />
    case 'ssl_analyze':
      return <SSLResultView data={data} selectedIds={selectedIds} onToggle={onToggle} />
    case 'classify':
      return <ClassifyResultView data={data} selectedIds={selectedIds} onToggle={onToggle} />
    case 'ai_research':
      return <AIResearchResultView data={data} selectedIds={selectedIds} onToggle={onToggle} />
    case 'whois':
      return <WhoisResultView data={data} />
    case 'dns':
      return <DNSResultView data={data} />
    default:
      return null // Use default table
  }
}

function AIResearchResultView({ data, selectedIds, onToggle }: { data: any[]; selectedIds: Set<string>; onToggle: (id: string) => void }) {
  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="divide-y divide-border/40">
        {data.map((row) => (
          <div
            key={row.id}
            onClick={() => onToggle(row.id)}
            className={clsx(
              'px-4 py-4 cursor-pointer transition-colors',
              selectedIds.has(row.id) ? 'bg-accent/5 hover:bg-accent/10' : 'hover:bg-raised/30'
            )}
          >
            <div className="flex items-start gap-4">
              <input
                type="checkbox"
                checked={selectedIds.has(row.id)}
                onChange={() => onToggle(row.id)}
                onClick={(e) => e.stopPropagation()}
                className="mt-1 w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer shrink-0"
              />
              <div className="flex-1 min-w-0">
                <div className="font-mono text-xs text-heading truncate">{row.url}</div>
                <AIResearchEvidence evidence={row.evidence} />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function AIResearchEvidence({ evidence }: { evidence?: string | null }) {
  if (!evidence) {
    return <div className="mt-2 text-sm text-muted">No AI research evidence saved yet.</div>
  }

  try {
    const parsed = JSON.parse(evidence)
    if (!parsed || typeof parsed !== 'object') {
      throw new Error('Invalid JSON evidence')
    }

    const whoisSignals = parsed.whois_signals && typeof parsed.whois_signals === 'object' ? parsed.whois_signals : null
    const entityProfile = parsed.entity_profile && typeof parsed.entity_profile === 'object' ? parsed.entity_profile : null
    const infrastructureSignals = parsed.infrastructure_signals && typeof parsed.infrastructure_signals === 'object' ? parsed.infrastructure_signals : null
    const attackSurface = Array.isArray(parsed.attack_surface) ? parsed.attack_surface : []
    const technologies = infrastructureSignals && Array.isArray(infrastructureSignals.observed_technologies)
      ? infrastructureSignals.observed_technologies
      : Array.isArray(parsed.observed_technologies)
        ? parsed.observed_technologies
        : []
    const securitySignals = Array.isArray(parsed.public_security_signals) ? parsed.public_security_signals : []
    const risks = Array.isArray(parsed.likely_risks) ? parsed.likely_risks : []
    const nextSteps = Array.isArray(parsed.next_steps)
      ? parsed.next_steps
      : Array.isArray(parsed.next_validation_steps)
        ? parsed.next_validation_steps
        : []
    const sources = Array.isArray(parsed.sources) ? parsed.sources : []

    return (
      <div className="mt-3 space-y-4">
        {parsed.target?.host && (
          <section>
            <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">Target</div>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs font-mono text-heading">
              <span>{String(parsed.target.host)}</span>
              {parsed.target?.url && <span className="text-muted">{String(parsed.target.url)}</span>}
            </div>
          </section>
        )}

        {parsed.summary && (
          <section>
            <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">Summary</div>
            <p className="mt-1 text-sm leading-6 text-text">{String(parsed.summary)}</p>
          </section>
        )}

        <StructuredObjectGrid title="Whois Signals" data={whoisSignals} />
        <StructuredObjectGrid title="Entity Profile" data={entityProfile} />
        <StructuredObjectGrid title="Infrastructure Signals" data={infrastructureSignals} hideArrayKeys={['observed_technologies', 'related_urls', 'historical_urls']} />

        <StructuredStringList title="Attack Surface" items={attackSurface} />
        <StructuredStringList title="Observed Technologies" items={technologies} />
        <StructuredStringList title="Public Security Signals" items={securitySignals} />
        {infrastructureSignals && (
          <>
            <StructuredStringList title="Related URLs" items={Array.isArray(infrastructureSignals.related_urls) ? infrastructureSignals.related_urls : []} />
            <StructuredStringList title="Historical URLs" items={Array.isArray(infrastructureSignals.historical_urls) ? infrastructureSignals.historical_urls : []} />
          </>
        )}

        {risks.length > 0 && (
          <section>
            <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">Likely Risks</div>
            <div className="mt-2 space-y-2">
              {risks.map((risk: any, index: number) => (
                <div key={`${risk?.title || 'risk'}-${index}`} className="rounded-lg border border-border bg-raised/20 px-3 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="text-sm font-medium text-heading">{String(risk?.title || 'Untitled risk')}</div>
                    {risk?.severity && (
                      <span className={clsx(
                        'rounded border px-1.5 py-0.5 text-[10px] font-mono uppercase',
                        severityClass(String(risk.severity))
                      )}>
                        {String(risk.severity)}
                      </span>
                    )}
                    {risk?.confidence && (
                      <span className="rounded border border-border px-1.5 py-0.5 text-[10px] font-mono uppercase text-muted">
                        {String(risk.confidence)} confidence
                      </span>
                    )}
                  </div>
                  {risk?.details && <p className="mt-2 text-sm leading-6 text-text">{String(risk.details)}</p>}
                </div>
              ))}
            </div>
          </section>
        )}

        <StructuredNextSteps title="Next Steps" items={nextSteps} />

        {sources.length > 0 && (
          <section>
            <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">Sources</div>
            <div className="mt-2 space-y-2">
              {sources.map((source: any, index: number) => (
                <div key={`${source?.url || 'source'}-${index}`} className="rounded-lg border border-border bg-raised/20 px-3 py-3">
                  <div className="text-sm font-medium text-heading">{String(source?.title || source?.url || 'Source')}</div>
                  {source?.url && (
                    <a
                      href={String(source.url)}
                      target="_blank"
                      rel="noreferrer"
                      className="mt-1 block break-all font-mono text-xs text-accent hover:underline"
                    >
                      {String(source.url)}
                    </a>
                  )}
                  {source?.reason && <p className="mt-2 text-sm leading-6 text-text">{String(source.reason)}</p>}
                </div>
              ))}
            </div>
          </section>
        )}

        <StructuredObjectGrid title="Metadata" data={parsed.metadata && typeof parsed.metadata === 'object' ? parsed.metadata : null} />
      </div>
    )
  } catch {
    return (
      <div className="mt-2 whitespace-pre-wrap text-sm leading-6 text-text">
        {evidence}
      </div>
    )
  }
}

function StructuredStringList({ title, items }: { title: string; items: unknown[] }) {
  const values = items.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
  if (values.length === 0) return null

  return (
    <section>
      <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">{title}</div>
      <div className="mt-2 space-y-2">
        {values.map((item, index) => (
          <div key={`${title}-${index}`} className="rounded-lg border border-border bg-raised/20 px-3 py-2.5 text-sm leading-6 text-text">
            {item}
          </div>
        ))}
      </div>
    </section>
  )
}

function StructuredNextSteps({ title, items }: { title: string; items: unknown[] }) {
  if (!Array.isArray(items) || items.length === 0) return null

  const objectItems = items.filter((item): item is Record<string, unknown> => !!item && typeof item === 'object' && !Array.isArray(item))
  if (objectItems.length === 0) {
    return <StructuredStringList title={title} items={items} />
  }

  return (
    <section>
      <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">{title}</div>
      <div className="mt-2 space-y-2">
        {objectItems.map((item, index) => (
          <div key={`${title}-${index}`} className="rounded-lg border border-border bg-raised/20 px-3 py-3">
            {(() => {
              const action = typeof item.action === 'string' ? item.action : typeof item.question === 'string' ? item.question : `Step ${index + 1}`
              const priority = typeof item.priority === 'string' ? item.priority : ''
              const details = typeof item.details === 'string' ? item.details : ''

              return (
                <>
            <div className="flex flex-wrap items-center gap-2">
              <div className="text-sm font-medium text-heading">{action}</div>
              {priority && (
                <span className={clsx('rounded border px-1.5 py-0.5 text-[10px] font-mono uppercase', severityClass(priority))}>
                  {priority}
                </span>
              )}
            </div>
            {details && <p className="mt-2 text-sm leading-6 text-text">{details}</p>}
                </>
              )
            })()}
          </div>
        ))}
      </div>
    </section>
  )
}

function StructuredObjectGrid({
  title,
  data,
  hideArrayKeys = [],
}: {
  title: string
  data: Record<string, unknown> | null
  hideArrayKeys?: string[]
}) {
  if (!data) return null

  const entries = Object.entries(data).filter(([key, value]) => {
    if (hideArrayKeys.includes(key) && Array.isArray(value)) return false
    if (Array.isArray(value)) return false
    return value != null && String(value).trim() !== ''
  })

  if (entries.length === 0) return null

  return (
    <section>
      <div className="text-[10px] font-mono uppercase tracking-[0.22em] text-muted">{title}</div>
      <div className="mt-2 grid gap-2 md:grid-cols-2">
        {entries.map(([key, value]) => (
          <div key={`${title}-${key}`} className="rounded-lg border border-border bg-raised/20 px-3 py-2.5">
            <div className="text-[10px] font-mono uppercase tracking-[0.18em] text-muted">
              {key.replace(/_/g, ' ')}
            </div>
            <div className="mt-1 text-sm leading-6 text-text break-words">
              {String(value)}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function severityClass(severity: string) {
  const normalized = severity.trim().toLowerCase()
  if (normalized === 'critical') return 'border-failed/30 bg-failed/10 text-failed'
  if (normalized === 'high') return 'border-high/30 bg-high/10 text-high'
  if (normalized === 'medium') return 'border-medium/30 bg-medium/10 text-medium'
  if (normalized === 'low') return 'border-completed/30 bg-completed/10 text-completed'
  return 'border-border text-muted'
}

// ─── WAF Detection ─────────────────────────────────────────────

function WAFResultView({ data, selectedIds, onToggle }: { data: any[]; selectedIds: Set<string>; onToggle: (id: string) => void }) {
  const withWAF = data.filter((r) => r.waf_detected)
  const withCDN = data.filter((r) => r.cdn_detected)
  const clean = data.filter((r) => !r.waf_detected && !r.cdn_detected)

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      {/* Summary */}
      <div className="px-4 py-3 border-b border-border bg-raised/30 flex items-center gap-4">
        <div className="flex items-center gap-2">
          <ShieldAlert size={14} className="text-failed" />
          <span className="font-mono text-xs text-heading">{withWAF.length} WAF detected</span>
        </div>
        <div className="flex items-center gap-2">
          <Globe size={14} className="text-medium" />
          <span className="font-mono text-xs text-heading">{withCDN.length} CDN detected</span>
        </div>
        <div className="flex items-center gap-2">
          <ShieldCheck size={14} className="text-completed" />
          <span className="font-mono text-xs text-muted">{clean.length} no protection</span>
        </div>
      </div>

      {/* Cards */}
      <div className="divide-y divide-border/40">
        {data.map((row) => (
          <div
            key={row.id}
            onClick={() => onToggle(row.id)}
            className={clsx(
              'flex items-center gap-4 px-4 py-3 cursor-pointer transition-colors',
              selectedIds.has(row.id) ? 'bg-accent/5 hover:bg-accent/10' : 'hover:bg-raised/30'
            )}
          >
            <input
              type="checkbox"
              checked={selectedIds.has(row.id)}
              onChange={() => onToggle(row.id)}
              onClick={(e) => e.stopPropagation()}
              className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer shrink-0"
            />

            {/* Icon */}
            <div className={clsx(
              'w-8 h-8 rounded-lg flex items-center justify-center shrink-0',
              row.waf_detected ? 'bg-failed/10' : row.cdn_detected ? 'bg-medium/10' : 'bg-completed/10'
            )}>
              {row.waf_detected ? <ShieldAlert size={16} className="text-failed" /> :
               row.cdn_detected ? <Globe size={16} className="text-medium" /> :
               <ShieldCheck size={16} className="text-completed" />}
            </div>

            {/* Target */}
            <div className="flex-1 min-w-0">
              <div className="font-mono text-xs text-heading truncate">{row.url}</div>
              <div className="flex items-center gap-2 mt-0.5">
                {row.waf_detected && (
                  <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-failed/10 text-failed border border-failed/20">
                    WAF: {row.waf_detected}
                  </span>
                )}
                {row.cdn_detected && (
                  <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-medium/10 text-medium border border-medium/20">
                    CDN: {row.cdn_detected}
                  </span>
                )}
                {!row.waf_detected && !row.cdn_detected && (
                  <span className="text-[10px] font-mono text-completed/70">No WAF/CDN detected</span>
                )}
              </div>
            </div>

            {/* Site type if set */}
            {row.site_type && row.site_type !== 'unknown' && (
              <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-accent/10 text-accent border border-accent/20 shrink-0">
                {row.site_type}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── SSL Analysis ──────────────────────────────────────────────

function SSLResultView({ data, selectedIds, onToggle }: { data: any[]; selectedIds: Set<string>; onToggle: (id: string) => void }) {
  const gradeColor = (grade: string) => {
    if (!grade) return 'text-muted'
    const g = grade.toUpperCase()
    if (g.startsWith('A')) return 'text-completed'
    if (g.startsWith('B')) return 'text-medium'
    if (g.startsWith('C')) return 'text-high'
    return 'text-failed'
  }

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="divide-y divide-border/40">
        {data.map((row) => (
          <div
            key={row.id}
            onClick={() => onToggle(row.id)}
            className={clsx(
              'flex items-center gap-4 px-4 py-3 cursor-pointer transition-colors',
              selectedIds.has(row.id) ? 'bg-accent/5 hover:bg-accent/10' : 'hover:bg-raised/30'
            )}
          >
            <input
              type="checkbox"
              checked={selectedIds.has(row.id)}
              onChange={() => onToggle(row.id)}
              onClick={(e) => e.stopPropagation()}
              className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer shrink-0"
            />

            <div className={clsx(
              'w-8 h-8 rounded-lg flex items-center justify-center shrink-0',
              row.ssl_grade ? 'bg-completed/10' : 'bg-failed/10'
            )}>
              {row.ssl_grade ? <Lock size={16} className="text-completed" /> : <Unlock size={16} className="text-failed" />}
            </div>

            <div className="flex-1 min-w-0">
              <div className="font-mono text-xs text-heading truncate">{row.url}</div>
              {row.ssl_details && (() => {
                try {
                  const d = typeof row.ssl_details === 'string' ? JSON.parse(row.ssl_details) : row.ssl_details
                  return (
                    <div className="flex items-center gap-2 mt-0.5 text-[10px] font-mono text-muted">
                      {d.issuer && <span>Issuer: {d.issuer}</span>}
                      {d.expires && <span>Exp: {d.expires}</span>}
                      {d.protocol && <span>{d.protocol}</span>}
                    </div>
                  )
                } catch { return null }
              })()}
            </div>

            {row.ssl_grade ? (
              <span className={clsx('text-lg font-bold font-mono', gradeColor(row.ssl_grade))}>
                {row.ssl_grade}
              </span>
            ) : (
              <span className="text-[10px] font-mono text-failed">No SSL</span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── Classify ──────────────────────────────────────────────────

function ClassifyResultView({ data, selectedIds, onToggle }: { data: any[]; selectedIds: Set<string>; onToggle: (id: string) => void }) {
  const typeIcon = (t: string) => {
    switch (t) {
      case 'spa': return <Eye size={14} className="text-accent" />
      case 'api': return <Server size={14} className="text-medium" />
      default: return <Globe size={14} className="text-muted" />
    }
  }

  const typeLabel: Record<string, string> = {
    spa: 'Single Page App', ssr: 'Server-Side Rendered', hybrid: 'Hybrid',
    classic: 'Classic/MPA', api: 'API', unknown: 'Unknown',
  }

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      {/* Summary chips */}
      <div className="px-4 py-3 border-b border-border bg-raised/30 flex items-center gap-2 flex-wrap">
        {Object.entries(
          data.reduce((acc: Record<string, number>, r) => {
            const t = r.site_type || 'unknown'
            acc[t] = (acc[t] || 0) + 1
            return acc
          }, {})
        ).map(([type, count]) => (
          <span key={type} className="text-[10px] font-mono px-2 py-1 rounded bg-accent/10 text-accent border border-accent/20">
            {typeLabel[type] || type}: {count as number}
          </span>
        ))}
      </div>

      <div className="divide-y divide-border/40">
        {data.map((row) => (
          <div
            key={row.id}
            onClick={() => onToggle(row.id)}
            className={clsx(
              'flex items-center gap-4 px-4 py-3 cursor-pointer transition-colors',
              selectedIds.has(row.id) ? 'bg-accent/5 hover:bg-accent/10' : 'hover:bg-raised/30'
            )}
          >
            <input
              type="checkbox"
              checked={selectedIds.has(row.id)}
              onChange={() => onToggle(row.id)}
              onClick={(e) => e.stopPropagation()}
              className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer shrink-0"
            />

            <div className="w-8 h-8 rounded-lg bg-raised flex items-center justify-center shrink-0">
              {typeIcon(row.site_type)}
            </div>

            <div className="flex-1 min-w-0">
              <div className="font-mono text-xs text-heading truncate">{row.url}</div>
              <div className="flex items-center gap-2 mt-0.5">
                <span className="text-[10px] font-mono text-accent">{typeLabel[row.site_type] || row.site_type}</span>
                {row.infra_type && row.infra_type !== 'unknown' && (
                  <span className="text-[10px] font-mono text-muted">{row.infra_type}</span>
                )}
                {row.waf_detected && (
                  <span className="text-[10px] font-mono px-1 rounded bg-failed/10 text-failed">WAF: {row.waf_detected}</span>
                )}
                {row.cdn_detected && (
                  <span className="text-[10px] font-mono px-1 rounded bg-medium/10 text-medium">CDN: {row.cdn_detected}</span>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── WHOIS ─────────────────────────────────────────────────────

function WhoisResultView({ data }: { data: any[] }) {
  if (!data.length) return null
  const r = data[0] // Usually single result

  const fields = [
    { label: 'Domain', value: r.domain },
    { label: 'Registrar', value: r.registrar },
    { label: 'Organization', value: r.org },
    { label: 'Created', value: r.created_date },
    { label: 'Expires', value: r.expiry_date },
    { label: 'Updated', value: r.updated_date },
    { label: 'Name Servers', value: r.name_servers },
    { label: 'Status', value: r.status },
    { label: 'Country', value: r.country },
    { label: 'Registrant', value: r.registrant_name },
    { label: 'Email', value: r.registrant_email },
  ].filter((f) => f.value)

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="px-4 py-3 border-b border-border bg-raised/30 flex items-center gap-2">
        <Wifi size={14} className="text-accent" />
        <span className="font-mono text-xs text-heading font-semibold">WHOIS — {r.domain}</span>
      </div>
      <div className="divide-y divide-border/30">
        {fields.map((f) => (
          <div key={f.label} className="flex px-4 py-2">
            <span className="text-[10px] font-mono text-muted uppercase tracking-wider w-28 shrink-0 pt-0.5">{f.label}</span>
            <span className="font-mono text-xs text-heading flex-1 break-all">{f.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── DNS ───────────────────────────────────────────────────────

function DNSResultView({ data }: { data: any[] }) {
  // Group by record type
  const grouped = data.reduce((acc: Record<string, any[]>, r) => {
    const t = r.record_type || 'OTHER'
    if (!acc[t]) acc[t] = []
    acc[t].push(r)
    return acc
  }, {})

  const typeOrder = ['A', 'AAAA', 'CNAME', 'MX', 'NS', 'TXT', 'SOA', 'SRV', 'PTR', 'CAA']
  const sortedTypes = [...Object.keys(grouped)].sort((a, b) => {
    const ai = typeOrder.indexOf(a)
    const bi = typeOrder.indexOf(b)
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
  })

  const typeColor: Record<string, string> = {
    A: 'text-accent', AAAA: 'text-accent', CNAME: 'text-medium',
    MX: 'text-completed', NS: 'text-high', TXT: 'text-muted',
    SOA: 'text-muted', SRV: 'text-medium', CAA: 'text-failed',
  }

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="divide-y divide-border/40">
        {sortedTypes.map((type) => (
          <div key={type}>
            <div className="px-4 py-2 bg-raised/30 flex items-center gap-2">
              <span className={clsx('font-mono text-[11px] font-bold', typeColor[type] || 'text-muted')}>{type}</span>
              <span className="text-[10px] font-mono text-muted">({grouped[type].length})</span>
            </div>
            {grouped[type].map((r: any, i: number) => (
              <div key={r.id || i} className="flex items-center px-4 py-1.5 hover:bg-raised/20">
                <span className="font-mono text-xs text-muted w-48 shrink-0 truncate">{r.host}</span>
                <span className="font-mono text-xs text-heading flex-1 break-all">{r.value}</span>
                {r.ttl && <span className="font-mono text-[10px] text-muted/50 shrink-0 ml-2">TTL {r.ttl}</span>}
                {r.priority != null && <span className="font-mono text-[10px] text-muted/50 shrink-0 ml-2">pri {r.priority}</span>}
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
