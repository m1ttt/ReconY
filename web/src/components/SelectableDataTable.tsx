import { useCallback, useRef } from 'react'
import { clsx } from 'clsx'
import { ArrowUp, ArrowDown, ArrowUpDown } from 'lucide-react'

interface Column<T> {
  key: string
  label: string
  render?: (row: T) => React.ReactNode
  className?: string
  mono?: boolean
  sortable?: boolean // default true
}

interface SelectableDataTableProps<T> {
  columns: Column<T>[]
  data: T[]
  loading?: boolean
  emptyMessage?: string
  selectedIds: Set<string>
  onToggle: (id: string) => void
  onSelectAll: (ids: string[]) => void
  onClearSelection: () => void
  getId: (row: T) => string
  sortKey?: string | null
  sortDir?: 'asc' | 'desc'
  onSort?: (key: string, dir: 'asc' | 'desc') => void
}

export function SelectableDataTable<T extends Record<string, any>>({
  columns, data, loading, emptyMessage = 'No data',
  selectedIds, onToggle, onSelectAll, onClearSelection, getId,
  sortKey, sortDir = 'asc', onSort,
}: SelectableDataTableProps<T>) {
  const lastClickedRef = useRef<number>(-1)

  const handleSort = (key: string) => {
    if (!onSort) return
    if (sortKey === key) {
      onSort(key, sortDir === 'asc' ? 'desc' : 'asc')
    } else {
      onSort(key, 'asc')
    }
  }

  const handleRowClick = useCallback((index: number, e: React.MouseEvent) => {
    const id = getId(data[index])

    if (e.shiftKey && lastClickedRef.current >= 0) {
      const start = Math.min(lastClickedRef.current, index)
      const end = Math.max(lastClickedRef.current, index)
      const rangeIds = data.slice(start, end + 1).map(getId)
      onSelectAll([...selectedIds, ...rangeIds])
    } else {
      onToggle(id)
    }
    lastClickedRef.current = index
  }, [data, selectedIds, onToggle, onSelectAll, getId])

  const allSelected = data.length > 0 && data.every((row) => selectedIds.has(getId(row)))

  const handleToggleAll = () => {
    if (allSelected) {
      onClearSelection()
    } else {
      onSelectAll(data.map(getId))
    }
  }

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
        <p className="text-muted font-mono text-sm">{emptyMessage}</p>
      </div>
    )
  }

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden flex flex-col">
      <div className="overflow-x-auto flex-1">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-raised/40">
              <th className="px-3 py-2.5 w-10">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={handleToggleAll}
                  className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer"
                />
              </th>
              {columns.map((col) => {
                const isSortable = col.sortable !== false && !!onSort
                const isActive = sortKey === col.key
                return (
                  <th
                    key={col.key}
                    onClick={isSortable ? () => handleSort(col.key) : undefined}
                    className={clsx(
                      'px-4 py-2.5 text-left text-[10px] font-mono font-semibold text-muted uppercase tracking-wider',
                      isSortable && 'cursor-pointer select-none hover:text-text transition-colors group'
                    )}
                  >
                    <span className="inline-flex items-center gap-1">
                      {col.label}
                      {isSortable && (
                        <span className={clsx(
                          'transition-opacity',
                          isActive ? 'opacity-100 text-accent' : 'opacity-0 group-hover:opacity-40'
                        )}>
                          {isActive && sortDir === 'asc' ? <ArrowUp size={10} /> :
                           isActive && sortDir === 'desc' ? <ArrowDown size={10} /> :
                           <ArrowUpDown size={10} />}
                        </span>
                      )}
                    </span>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {data.map((row, i) => {
              const id = getId(row)
              const isSelected = selectedIds.has(id)
              return (
                <tr
                  key={id}
                  onClick={(e) => handleRowClick(i, e)}
                  className={clsx(
                    'border-b border-border/50 transition-colors cursor-pointer',
                    isSelected
                      ? 'bg-accent/5 hover:bg-accent/10'
                      : i % 2 === 0 ? 'bg-transparent hover:bg-raised/30' : 'bg-raised/10 hover:bg-raised/30'
                  )}
                >
                  <td className="px-3 py-2.5 w-10">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggle(id)}
                      onClick={(e) => e.stopPropagation()}
                      className="w-3.5 h-3.5 rounded border-border bg-deep accent-accent cursor-pointer"
                    />
                  </td>
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={clsx(
                        'px-4 py-2.5 text-sm',
                        col.mono && 'font-mono text-xs',
                        col.className
                      )}
                    >
                      {col.render ? col.render(row) : String(row[col.key] ?? '—')}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
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
