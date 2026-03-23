import { useState } from 'react'
import { Wrench, ChevronDown, ChevronRight } from 'lucide-react'
import { clsx } from 'clsx'

interface Props {
  config: any
  onChange: (patch: any) => void
}

const inputClass = 'w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20'
const labelClass = 'text-[11px] font-mono text-muted uppercase tracking-wider'

const TOOL_LIST = [
  'subfinder', 'amass', 'puredns', 'nmap', 'shodan', 'censys',
  'httpx', 'waf_detect', 'ssl_analyze', 'classify',
  'katana', 'ffuf', 'feroxbuster', 'gowitness', 'cmseek', 'paramspider', 'jsluice', 'secretfinder', 'static-analysis',
  'bucket_enum', 'gitdork', 'js_secrets',
  'nuclei',
]

export function ToolConfigSection({ config, onChange }: Props) {
  const tools = config.tools || {}
  const [expanded, setExpanded] = useState<string | null>(null)

  const updateTool = (name: string, field: string, value: any) => {
    const current = tools[name] || {}
    onChange({
      tools: {
        ...tools,
        [name]: { ...current, [field]: value },
      },
    })
  }

  const toggleEnabled = (name: string) => {
    const current = tools[name] || {}
    const currentEnabled = current.enabled !== false
    updateTool(name, 'enabled', !currentEnabled)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <Wrench size={16} className="text-accent" />
        <h2 className="text-sm font-semibold text-heading uppercase tracking-wider">Tool Configuration</h2>
      </div>

      <p className="text-xs text-muted -mt-3">
        Per-tool settings: threads, timeouts, rate limits, and extra arguments.
      </p>

      <div className="space-y-1">
        {TOOL_LIST.map((name) => {
          const tc = tools[name] || {}
          const enabled = tc.enabled !== false
          const isExpanded = expanded === name

          return (
            <div
              key={name}
              className="bg-surface border border-border rounded-lg overflow-hidden transition-colors hover:border-border-bright"
            >
              {/* Tool Header */}
              <div className="flex items-center gap-3 px-4 py-2.5">
                <div
                  onClick={() => toggleEnabled(name)}
                  className={clsx(
                    'w-8 h-4.5 rounded-full transition-colors relative cursor-pointer shrink-0',
                    enabled ? 'bg-accent' : 'bg-elevated'
                  )}
                >
                  <div className={clsx(
                    'absolute top-0.5 w-3.5 h-3.5 rounded-full bg-white transition-transform',
                    enabled ? 'translate-x-4' : 'translate-x-0.5'
                  )} />
                </div>

                <span className={clsx(
                  'font-mono text-sm flex-1',
                  enabled ? 'text-heading' : 'text-muted'
                )}>
                  {name}
                </span>

                {(tc.threads || tc.timeout || tc.rate_limit) && (
                  <div className="flex items-center gap-2 text-[10px] font-mono text-muted">
                    {tc.threads && <span>t:{tc.threads}</span>}
                    {tc.timeout && <span>{tc.timeout}</span>}
                    {tc.rate_limit && <span>rl:{tc.rate_limit}</span>}
                  </div>
                )}

                <button
                  onClick={() => setExpanded(isExpanded ? null : name)}
                  className="p-1 rounded text-muted hover:text-accent transition-colors"
                >
                  {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </button>
              </div>

              {/* Expanded Config */}
              {isExpanded && (
                <div className="px-4 pb-4 pt-1 border-t border-border animate-fade-in">
                  <div className="grid grid-cols-3 gap-4 mt-3">
                    <div>
                      <label className={labelClass}>Threads</label>
                      <input
                        type="number" value={tc.threads || ''} placeholder="10" min={1}
                        onChange={(e) => updateTool(name, 'threads', parseInt(e.target.value) || null)}
                        className={inputClass}
                      />
                    </div>
                    <div>
                      <label className={labelClass}>Timeout</label>
                      <input
                        type="text" value={tc.timeout || ''} placeholder="30m"
                        onChange={(e) => updateTool(name, 'timeout', e.target.value || undefined)}
                        className={inputClass}
                      />
                    </div>
                    <div>
                      <label className={labelClass}>Rate Limit</label>
                      <input
                        type="number" value={tc.rate_limit || ''} placeholder="0" min={0}
                        onChange={(e) => updateTool(name, 'rate_limit', parseInt(e.target.value) || null)}
                        className={inputClass}
                      />
                    </div>
                  </div>
                  <div className="mt-3">
                    <label className={labelClass}>Extra Arguments</label>
                    <input
                      type="text"
                      value={(tc.extra_args || []).join(' ')}
                      placeholder="-flag1 -flag2 value"
                      onChange={(e) => updateTool(name, 'extra_args', e.target.value ? e.target.value.split(' ') : [])}
                      className={inputClass}
                    />
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
