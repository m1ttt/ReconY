import { clsx } from 'clsx'
import { Check, X, Loader, Play } from 'lucide-react'
import type { ReconStep } from '../store/reconSession'

interface Props {
  steps: ReconStep[]
  currentStepIndex: number
  onSelectStep: (index: number) => void
}

const statusIcons = {
  running: Loader,
  completed: Check,
  failed: X,
}

const statusColors = {
  running: 'text-running border-running/30 bg-running/10',
  completed: 'text-completed border-completed/30 bg-completed/10',
  failed: 'text-failed border-failed/30 bg-failed/10',
}

export function SessionTimeline({ steps, currentStepIndex, onSelectStep }: Props) {
  if (steps.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full px-3 py-8">
        <Play size={20} className="text-muted/20 mb-3" />
        <p className="text-[10px] font-mono text-muted/50 text-center leading-relaxed">
          Pick a tool to start your recon chain
        </p>
      </div>
    )
  }

  return (
    <div className="py-3 px-2 space-y-1 overflow-y-auto">
      <div className="text-[9px] font-mono text-muted uppercase tracking-wider px-2 mb-2">
        Session
      </div>
      {steps.map((step, i) => {
        const Icon = statusIcons[step.status]
        const isCurrent = i === currentStepIndex

        return (
          <button
            key={step.id}
            onClick={() => onSelectStep(i)}
            className={clsx(
              'w-full flex items-center gap-2 px-2.5 py-2 rounded-md text-left transition-all',
              isCurrent
                ? 'bg-accent/10 border border-accent/20'
                : 'hover:bg-raised/50 border border-transparent'
            )}
          >
            {/* Status dot */}
            <div className={clsx(
              'w-5 h-5 rounded flex items-center justify-center shrink-0 border',
              statusColors[step.status]
            )}>
              <Icon size={10} className={step.status === 'running' ? 'animate-spin' : ''} />
            </div>

            {/* Info */}
            <div className="min-w-0 flex-1">
              <div className={clsx(
                'text-[11px] font-mono truncate',
                isCurrent ? 'text-accent' : 'text-heading'
              )}>
                {step.toolName}
              </div>
              <div className="text-[9px] font-mono text-muted flex items-center gap-1.5">
                {step.resultCount > 0 && (
                  <span className="text-accent/70">{step.resultCount}</span>
                )}
                {step.targetCount && (
                  <span>on {step.targetCount}</span>
                )}
              </div>
            </div>

            {/* Connector line */}
            {i < steps.length - 1 && (
              <div className="absolute left-[21px] top-full w-px h-1 bg-border" />
            )}
          </button>
        )
      })}
    </div>
  )
}
