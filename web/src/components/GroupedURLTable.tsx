import { useState, useMemo, useCallback, useRef } from 'react'
import { clsx } from 'clsx'
import { ChevronRight, ChevronDown, FolderOpen, Folder, List, Network } from 'lucide-react'

interface Props {
  data: any[]
  selectedIds: Set<string>
  onToggle: (id: string) => void
  onSelectAll: (ids: string[]) => void
  onClearSelection: () => void
  loading?: boolean
}

interface URLGroup {
  path: string        // e.g. "/admin/api"
  depth: number
  urls: any[]         // leaf URLs in this exact directory
  children: URLGroup[]
  allIds: string[]    // all IDs in this group + descendants
  totalCount: number
}

function parseURLParts(urlStr: string): { host: string; path: string; params: string[] } {
  try {
    const u = new URL(urlStr)
    const params = [...u.searchParams.keys()]
    return { host: u.origin, path: u.pathname, params }
  } catch {
    return { host: '', path: urlStr, params: [] }
  }
}

// Compat alias used in grouping logic
function parseURLPath(urlStr: string): { host: string; path: string } {
  const { host, path } = parseURLParts(urlStr)
  return { host, path }
}

// Build a tree of URL groups from flat URL list
function buildGroupTree(data: any[]): Map<string, URLGroup> {
  // Group by host first
  const byHost = new Map<string, any[]>()
  for (const row of data) {
    const { host } = parseURLPath(row.url)
    if (!byHost.has(host)) byHost.set(host, [])
    byHost.get(host)!.push(row)
  }

  const hostGroups = new Map<string, URLGroup>()

  for (const [host, urls] of byHost) {
    // Group URLs by their directory path
    const dirMap = new Map<string, any[]>()

    for (const row of urls) {
      const { path } = parseURLPath(row.url)
      // Get the directory: /admin/api/users → /admin/api
      const lastSlash = path.lastIndexOf('/')
      const dir = lastSlash <= 0 ? '/' : path.substring(0, lastSlash)
      if (!dirMap.has(dir)) dirMap.set(dir, [])
      dirMap.get(dir)!.push(row)
    }

    // Sort directories alphabetically
    const sortedDirs = [...dirMap.keys()].sort()

    // Build flat group list (not nested tree — simpler, more scannable)
    const children: URLGroup[] = sortedDirs.map((dir) => {
      const dirURLs = dirMap.get(dir)!.sort((a: any, b: any) =>
        (a.url as string).localeCompare(b.url as string)
      )
      return {
        path: dir,
        depth: dir.split('/').filter(Boolean).length,
        urls: dirURLs,
        children: [],
        allIds: dirURLs.map((u: any) => u.id),
        totalCount: dirURLs.length,
      }
    })

    hostGroups.set(host, {
      path: host,
      depth: 0,
      urls: [],
      children,
      allIds: urls.map((u: any) => u.id),
      totalCount: urls.length,
    })
  }

  return hostGroups
}

function StatusBadge({ code }: { code: number }) {
  if (!code) return <span className="text-muted font-mono text-[10px]">—</span>
  const color = code >= 200 && code < 300 ? 'text-completed bg-completed/10'
    : code >= 300 && code < 400 ? 'text-medium bg-medium/10'
    : code >= 400 && code < 500 ? 'text-failed bg-failed/10'
    : code >= 500 ? 'text-critical bg-critical/10'
    : 'text-muted bg-muted/10'
  return (
    <span className={clsx('font-mono text-[10px] px-1.5 py-0.5 rounded', color)}>
      {code}
    </span>
  )
}

export function GroupedURLTable({ data, selectedIds, onToggle, onSelectAll, onClearSelection, loading }: Props) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set(['__all__']))
  const [viewMode, setViewMode] = useState<'grouped' | 'flat'>('grouped')
  const lastClickedRef = useRef<number>(-1)

  const hostGroups = useMemo(() => buildGroupTree(data), [data])

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  const expandAll = () => {
    const all = new Set(['__all__'])
    for (const [host, group] of hostGroups) {
      all.add(host)
      for (const child of group.children) all.add(host + child.path)
    }
    setExpandedGroups(all)
  }

  const collapseAll = () => setExpandedGroups(new Set())

  const selectGroup = (ids: string[]) => {
    onSelectAll([...selectedIds, ...ids])
  }

  const handleFlatRowClick = useCallback((index: number, e: React.MouseEvent) => {
    const id = data[index]?.id
    if (!id) return
    if (e.shiftKey && lastClickedRef.current >= 0) {
      const start = Math.min(lastClickedRef.current, index)
      const end = Math.max(lastClickedRef.current, index)
      const rangeIds = data.slice(start, end + 1).map((r: any) => r.id)
      onSelectAll([...selectedIds, ...rangeIds])
    } else {
      onToggle(id)
    }
    lastClickedRef.current = index
  }, [data, selectedIds, onToggle, onSelectAll])

  if (loading) {
    return (
      <div className="bg-surface border border-border rounded-lg p-12 text-center">
        <div className="inline-flex items-center gap-2 text-muted">
          <span className="w-2 h-2 rounded-full bg-accent animate-pulse" />
          <span className="font-mono text-sm">Loading...</span>
        </div>
      </div>
    )
  }

  if (!data.length) {
    return (
      <div className="bg-surface border border-border rounded-lg p-12 text-center">
        <p className="text-muted font-mono text-sm">No URLs found</p>
      </div>
    )
  }

  const allSelected = data.length > 0 && data.every((r: any) => selectedIds.has(r.id))

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden flex flex-col">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-raised/30">
        <div className="flex items-center gap-1">
          <button
            onClick={() => setViewMode('grouped')}
            className={clsx(
              'p-1.5 rounded transition-colors',
              viewMode === 'grouped' ? 'bg-accent/15 text-accent' : 'text-muted hover:text-text'
            )}
            title="Group by path"
          >
            <Network size={13} />
          </button>
          <button
            onClick={() => setViewMode('flat')}
            className={clsx(
              'p-1.5 rounded transition-colors',
              viewMode === 'flat' ? 'bg-accent/15 text-accent' : 'text-muted hover:text-text'
            )}
            title="Flat list"
          >
            <List size={13} />
          </button>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-muted">{data.length} urls</span>
          {viewMode === 'grouped' && (
            <>
              <button onClick={expandAll} className="text-[10px] font-mono text-muted hover:text-accent transition-colors">
                expand
              </button>
              <button onClick={collapseAll} className="text-[10px] font-mono text-muted hover:text-accent transition-colors">
                collapse
              </button>
            </>
          )}
          <input
            type="checkbox"
            checked={allSelected}
            onChange={() => allSelected ? onClearSelection() : onSelectAll(data.map((r: any) => r.id))}
            className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer"
            title="Select all"
          />
        </div>
      </div>

      {/* Content */}
      <div className="overflow-y-auto flex-1">
        {viewMode === 'flat' ? (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-raised/40">
                <th className="px-3 py-2 w-8" />
                <th className="px-3 py-2 text-left text-[10px] font-mono font-semibold text-muted uppercase tracking-wider">URL</th>
                <th className="px-3 py-2 text-left text-[10px] font-mono font-semibold text-muted uppercase tracking-wider w-16">Status</th>
                <th className="px-3 py-2 text-left text-[10px] font-mono font-semibold text-muted uppercase tracking-wider w-20">Source</th>
              </tr>
            </thead>
            <tbody>
              {data.map((row: any, i: number) => (
                <tr
                  key={row.id}
                  onClick={(e) => handleFlatRowClick(i, e)}
                  className={clsx(
                    'border-b border-border/50 transition-colors cursor-pointer',
                    selectedIds.has(row.id)
                      ? 'bg-accent/5 hover:bg-accent/10'
                      : i % 2 === 0 ? 'bg-transparent hover:bg-raised/30' : 'bg-raised/10 hover:bg-raised/30'
                  )}
                >
                  <td className="px-3 py-1.5">
                    <input
                      type="checkbox"
                      checked={selectedIds.has(row.id)}
                      onChange={() => onToggle(row.id)}
                      onClick={(e) => e.stopPropagation()}
                      className="w-3 h-3 rounded border-border bg-deep accent-accent cursor-pointer"
                    />
                  </td>
                  <td className="px-3 py-1.5 font-mono text-xs">
                    {(() => {
                      const { params } = parseURLParts(row.url)
                      const base = params.length > 0 ? row.url.split('?')[0] : row.url
                      return (
                        <span className="inline-flex items-center gap-1.5 flex-wrap">
                          {base}
                          {params.map((p: string) => (
                            <span key={p} className="text-[9px] px-1 py-0 rounded bg-medium/15 text-medium border border-medium/20">
                              {p}
                            </span>
                          ))}
                        </span>
                      )
                    })()}
                  </td>
                  <td className="px-3 py-1.5"><StatusBadge code={row.status_code} /></td>
                  <td className="px-3 py-1.5 font-mono text-[10px] text-muted">{row.source}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="divide-y divide-border/30">
            {[...hostGroups.entries()].map(([host, hostGroup]) => {
              const hostExpanded = expandedGroups.has(host)
              const hostSelectedCount = hostGroup.allIds.filter((id) => selectedIds.has(id)).length

              return (
                <div key={host}>
                  {/* Host header */}
                  <div
                    className="flex items-center gap-2 px-3 py-2 bg-raised/50 cursor-pointer hover:bg-raised/80 transition-colors"
                    onClick={() => toggleGroup(host)}
                  >
                    {hostExpanded ? <ChevronDown size={12} className="text-muted shrink-0" /> : <ChevronRight size={12} className="text-muted shrink-0" />}
                    <span className="font-mono text-xs text-accent font-semibold">{host}</span>
                    <span className="text-[10px] font-mono text-muted">({hostGroup.totalCount})</span>
                    {hostSelectedCount > 0 && (
                      <span className="text-[10px] font-mono text-accent/70 ml-auto">{hostSelectedCount} sel</span>
                    )}
                    <button
                      onClick={(e) => { e.stopPropagation(); selectGroup(hostGroup.allIds) }}
                      className="text-[10px] font-mono text-muted hover:text-accent transition-colors ml-1"
                    >
                      all
                    </button>
                  </div>

                  {hostExpanded && hostGroup.children.map((dirGroup) => {
                    const dirKey = host + dirGroup.path
                    const dirExpanded = expandedGroups.has(dirKey)
                    const dirSelectedCount = dirGroup.allIds.filter((id) => selectedIds.has(id)).length

                    return (
                      <div key={dirKey}>
                        {/* Directory header */}
                        <div
                          className="flex items-center gap-2 px-3 py-1.5 cursor-pointer hover:bg-raised/40 transition-colors"
                          style={{ paddingLeft: `${12 + 16}px` }}
                          onClick={() => toggleGroup(dirKey)}
                        >
                          {dirExpanded
                            ? <><ChevronDown size={11} className="text-muted/60 shrink-0" /><FolderOpen size={12} className="text-accent/50 shrink-0" /></>
                            : <><ChevronRight size={11} className="text-muted/60 shrink-0" /><Folder size={12} className="text-muted/50 shrink-0" /></>
                          }
                          <span className="font-mono text-[11px] text-heading">{dirGroup.path}/</span>
                          <span className="text-[10px] font-mono text-muted">({dirGroup.totalCount})</span>

                          {/* Status code summary */}
                          <div className="flex items-center gap-1 ml-auto">
                            {(() => {
                              const codes = new Map<number, number>()
                              for (const u of dirGroup.urls) {
                                const c = u.status_code || 0
                                if (c > 0) codes.set(c, (codes.get(c) || 0) + 1)
                              }
                              return [...codes.entries()].sort(([a], [b]) => a - b).map(([code, count]) => (
                                <span key={code} className="inline-flex items-center gap-0.5">
                                  <StatusBadge code={code} />
                                  {count > 1 && <span className="text-[9px] font-mono text-muted">{count}</span>}
                                </span>
                              ))
                            })()}
                          </div>

                          {dirSelectedCount > 0 && (
                            <span className="text-[10px] font-mono text-accent/70">{dirSelectedCount}</span>
                          )}
                          <button
                            onClick={(e) => { e.stopPropagation(); selectGroup(dirGroup.allIds) }}
                            className="text-[10px] font-mono text-muted hover:text-accent transition-colors"
                          >
                            all
                          </button>
                        </div>

                        {/* URLs in this directory */}
                        {dirExpanded && dirGroup.urls.map((row: any) => {
                          const { path, params } = parseURLParts(row.url)
                          const filename = path.substring(path.lastIndexOf('/') + 1) || '/'
                          const isSelected = selectedIds.has(row.id)

                          return (
                            <div
                              key={row.id}
                              onClick={() => onToggle(row.id)}
                              className={clsx(
                                'flex items-center gap-2 px-3 py-1 cursor-pointer transition-colors',
                                isSelected
                                  ? 'bg-accent/5 hover:bg-accent/10'
                                  : 'hover:bg-raised/30'
                              )}
                              style={{ paddingLeft: `${12 + 16 + 24}px` }}
                            >
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={() => onToggle(row.id)}
                                onClick={(e) => e.stopPropagation()}
                                className="w-3 h-3 rounded border-border bg-deep accent-accent cursor-pointer shrink-0"
                              />
                              <span className="font-mono text-[11px] text-text truncate flex-1 inline-flex items-center gap-1.5">
                                {filename}
                                {params.length > 0 && (
                                  <span className="inline-flex items-center gap-0.5 flex-shrink-0">
                                    {params.map((p) => (
                                      <span key={p} className="text-[9px] font-mono px-1 py-0 rounded bg-medium/15 text-medium border border-medium/20">
                                        {p}
                                      </span>
                                    ))}
                                  </span>
                                )}
                              </span>
                              <StatusBadge code={row.status_code} />
                              <span className="font-mono text-[9px] text-muted/50 w-12 text-right shrink-0">
                                {row.content_length ? `${(row.content_length / 1024).toFixed(1)}k` : ''}
                              </span>
                              <span className="font-mono text-[9px] text-muted/40 w-16 text-right shrink-0 truncate">
                                {row.source}
                              </span>
                            </div>
                          )
                        })}
                      </div>
                    )
                  })}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Selection bar */}
      {selectedIds.size > 0 && (
        <div className="border-t border-accent/20 bg-accent/5 px-4 py-2.5 flex items-center justify-between animate-fade-in">
          <span className="text-xs font-mono text-accent">
            {selectedIds.size} of {data.length} selected
          </span>
          <button
            onClick={onClearSelection}
            className="text-xs font-mono text-muted hover:text-text transition-colors"
          >
            Clear
          </button>
        </div>
      )}
    </div>
  )
}
