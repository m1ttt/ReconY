import { clsx } from 'clsx'

interface Column<T> {
  key: string
  label: string
  render?: (row: T) => React.ReactNode
  className?: string
  mono?: boolean
}

interface DataTableProps<T> {
  columns: Column<T>[]
  data: T[]
  loading?: boolean
  emptyMessage?: string
}

export function DataTable<T extends Record<string, any>>({
  columns, data, loading, emptyMessage = 'No data'
}: DataTableProps<T>) {
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
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-raised/40">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className="px-4 py-2.5 text-left text-[10px] font-mono font-semibold text-muted uppercase tracking-wider"
                >
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row, i) => (
              <tr
                key={row.id || i}
                className={clsx(
                  'border-b border-border/50 hover:bg-raised/30 transition-colors',
                  i % 2 === 0 ? 'bg-transparent' : 'bg-raised/10'
                )}
                style={{ animationDelay: `${i * 20}ms` }}
              >
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
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
