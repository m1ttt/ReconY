import { useState, useEffect } from 'react'
import { clsx } from 'clsx'
import { Play, Shield, AlertTriangle, ChevronDown, ChevronRight } from 'lucide-react'
import { api, type ToolRegistryEntry } from '../api/client'

interface Props {
  selectedDataType: string | null
  selectedCount: number
  onRunTool: (toolName: string) => void
  onSetupAuth: () => void
  runningCount: number
}

// Map data types to what can accept them
function getAcceptableTools(
  registry: Record<number, ToolRegistryEntry[]>,
  dataType: string | null
): { primary: ToolRegistryEntry[]; secondary: ToolRegistryEntry[] } {
  const all = Object.values(registry).flat()

  if (!dataType) {
    const primary = all.filter((t) => t.accepts.includes('domain'))
    return { primary, secondary: [] }
  }

  // Show tools that accept the current data type as primary.
  // Also include tools that accept 'domain' since BuildHTTPTargets always
  // falls back to the domain — these tools always work.
  const primary = all.filter((t) =>
    t.accepts.includes(dataType) || t.accepts.includes('domain')
  )
  // Only truly unrelated tools go to secondary
  const secondary = all.filter(
    (t) => !t.accepts.includes(dataType) && !t.accepts.includes('domain')
  )
  return { primary, secondary }
}

export function ActionPanel({ selectedDataType, selectedCount, onRunTool, onSetupAuth, runningCount }: Props) {
  const [registry, setRegistry] = useState<Record<number, ToolRegistryEntry[]>>({})
  const [selectedTool, setSelectedTool] = useState<string | null>(null)
  const [showSecondary, setShowSecondary] = useState(false)

  useEffect(() => {
    api.getToolRegistry().then(setRegistry)
  }, [])

  const { primary, secondary } = getAcceptableTools(registry, selectedDataType)

  const handleRun = () => {
    if (selectedTool) {
      onRunTool(selectedTool)
      setSelectedTool(null)
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-[10px] font-mono text-muted uppercase tracking-wider">
          Next Action
        </h3>
        {selectedCount > 0 && (
          <p className="text-xs font-mono text-accent mt-1">
            {selectedCount} {selectedDataType || 'items'} selected
          </p>
        )}
        {runningCount > 0 && (
          <p className="text-[10px] font-mono text-running mt-1">
            {runningCount} scan{runningCount === 1 ? '' : 's'} running
          </p>
        )}
        {selectedCount === 0 && selectedDataType && (
          <p className="text-[10px] font-mono text-muted/60 mt-1">
            Select items or run on all
          </p>
        )}
      </div>

      {/* Tool List */}
      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-0.5">
        {/* Auth action */}
        {selectedDataType === 'subdomains' && selectedCount > 0 && (
          <button
            onClick={onSetupAuth}
            className="w-full flex items-center gap-2.5 px-3 py-2 rounded-md text-left transition-all hover:bg-accent/10 border border-transparent hover:border-accent/20"
          >
            <Shield size={13} className="text-accent shrink-0" />
            <div className="min-w-0 flex-1">
              <div className="text-[11px] font-mono text-accent">Setup Auth</div>
              <div className="text-[9px] font-mono text-muted">Configure credentials for targets</div>
            </div>
          </button>
        )}

        {/* Primary tools */}
        {primary.map((tool) => (
          <button
            key={tool.name}
            onClick={() => setSelectedTool(selectedTool === tool.name ? null : tool.name)}
            disabled={!tool.available}
            className={clsx(
              'w-full flex items-center gap-2.5 px-3 py-2 rounded-md text-left transition-all border',
              selectedTool === tool.name
                ? 'bg-accent/10 border-accent/30'
                : 'border-transparent hover:bg-raised/50 hover:border-border',
              !tool.available && 'opacity-40',
              !tool.available && 'cursor-not-allowed'
            )}
          >
            <div className={clsx(
              'w-1.5 h-1.5 rounded-full shrink-0',
              tool.available ? 'bg-completed' : 'bg-failed'
            )} />
            <div className="min-w-0 flex-1">
              <div className={clsx(
                'text-[11px] font-mono',
                selectedTool === tool.name ? 'text-accent' : 'text-heading'
              )}>
                {tool.name}
              </div>
              <div className="text-[9px] font-mono text-muted">
                {tool.phase_name} &middot; {tool.produces.join(', ')}
              </div>
            </div>
            {!tool.available && (
              <AlertTriangle size={11} className="text-failed shrink-0" />
            )}
          </button>
        ))}

        {/* Secondary tools toggle */}
        {secondary.length > 0 && (
          <>
            <button
              onClick={() => setShowSecondary(!showSecondary)}
              className="w-full flex items-center gap-2 px-3 py-1.5 text-[10px] font-mono text-muted hover:text-text transition-colors"
            >
              {showSecondary ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
              Other tools ({secondary.length})
            </button>
            {showSecondary && secondary.map((tool) => (
              <button
                key={tool.name}
                onClick={() => setSelectedTool(selectedTool === tool.name ? null : tool.name)}
                disabled={!tool.available}
                className={clsx(
                  'w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md text-left transition-all border opacity-60',
                  selectedTool === tool.name
                    ? 'bg-accent/10 border-accent/30 opacity-100'
                    : 'border-transparent hover:bg-raised/50'
                )}
              >
                <div className={clsx(
                  'w-1.5 h-1.5 rounded-full shrink-0',
                  tool.available ? 'bg-completed/50' : 'bg-failed/50'
                )} />
                <div className="text-[11px] font-mono text-muted">{tool.name}</div>
              </button>
            ))}
          </>
        )}
      </div>

      {/* Run Button */}
      <div className="px-3 py-3 border-t border-border">
        <button
          onClick={handleRun}
          disabled={!selectedTool}
          className={clsx(
            'w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold transition-all',
            selectedTool
              ? 'bg-accent text-void hover:bg-accent-dim'
              : 'bg-elevated text-muted cursor-not-allowed'
          )}
        >
          {runningCount > 0 ? (
            <>
              <Play size={14} />
              {selectedTool ? `Run ${selectedTool} (parallel)` : 'Select a tool'}
            </>
          ) : (
            <>
              <Play size={14} />
              {selectedTool ? `Run ${selectedTool}` : 'Select a tool'}
            </>
          )}
        </button>
      </div>
    </div>
  )
}
