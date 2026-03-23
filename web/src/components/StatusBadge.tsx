import { clsx } from 'clsx'
import type { Severity } from '../types'

export function SeverityBadge({ severity }: { severity: Severity }) {
  const colors: Record<string, string> = {
    critical: 'bg-critical/15 text-critical border-critical/30',
    high: 'bg-high/15 text-high border-high/30',
    medium: 'bg-medium/15 text-medium border-medium/30',
    low: 'bg-low/15 text-low border-low/30',
    info: 'bg-info/15 text-info border-info/30',
  }

  return (
    <span className={clsx(
      'inline-flex items-center px-2 py-0.5 rounded text-[11px] font-mono font-semibold uppercase tracking-wider border',
      colors[severity] || colors.info
    )}>
      {severity}
    </span>
  )
}

export function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    running: 'bg-running/15 text-running border-running/30',
    completed: 'bg-completed/15 text-completed border-completed/30',
    failed: 'bg-failed/15 text-failed border-failed/30',
    queued: 'bg-queued/15 text-queued border-queued/30',
    cancelled: 'bg-medium/15 text-medium border-medium/30',
  }

  return (
    <span className={clsx(
      'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-mono font-medium uppercase tracking-wider border',
      colors[status] || colors.queued
    )}>
      {status === 'running' && (
        <span className="w-1.5 h-1.5 rounded-full bg-running animate-pulse" />
      )}
      {status}
    </span>
  )
}

export function StatCard({ label, value, accent }: { label: string; value: number | string; accent?: boolean }) {
  return (
    <div className="bg-surface border border-border rounded-lg p-4 hover:border-border-bright transition-colors">
      <p className="text-[11px] font-mono text-muted uppercase tracking-wider mb-1">{label}</p>
      <p className={clsx(
        'text-2xl font-bold font-mono',
        accent ? 'text-accent' : 'text-heading'
      )}>
        {value}
      </p>
    </div>
  )
}
