import { Settings } from 'lucide-react'

interface Props {
  config: any
  onChange: (patch: any) => void
}

const inputClass = 'w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20'
const labelClass = 'text-[11px] font-mono text-muted uppercase tracking-wider'

export function GeneralSection({ config, onChange }: Props) {
  const g = config.general || {}

  const update = (field: string, value: any) => {
    onChange({ general: { ...g, [field]: value } })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <Settings size={16} className="text-accent" />
        <h2 className="text-sm font-semibold text-heading uppercase tracking-wider">General</h2>
      </div>

      <div className="grid grid-cols-2 gap-5">
        <div>
          <label className={labelClass}>Database Path</label>
          <input
            type="text" value={g.db_path || ''} placeholder="~/.reconx/reconx.db"
            onChange={(e) => update('db_path', e.target.value)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>Screenshots Directory</label>
          <input
            type="text" value={g.screenshots_dir || ''} placeholder="~/.reconx/screenshots"
            onChange={(e) => update('screenshots_dir', e.target.value)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>SecLists Path</label>
          <input
            type="text" value={g.seclists_path || ''} placeholder="/opt/SecLists"
            onChange={(e) => update('seclists_path', e.target.value)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>Default Workflow</label>
          <input
            type="text" value={g.default_workflow || ''} placeholder="full"
            onChange={(e) => update('default_workflow', e.target.value)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>API Listen Address</label>
          <input
            type="text" value={g.api_listen_addr || ''} placeholder=":8420"
            onChange={(e) => update('api_listen_addr', e.target.value)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>Max Concurrent Tools</label>
          <input
            type="number" value={g.max_concurrent_tools || ''} placeholder="4" min={1} max={20}
            onChange={(e) => update('max_concurrent_tools', parseInt(e.target.value) || 0)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>Rate Limit per Host (req/s)</label>
          <input
            type="number" value={g.rate_limit_per_host || ''} placeholder="10" min={0}
            onChange={(e) => update('rate_limit_per_host', parseInt(e.target.value) || 0)}
            className={inputClass}
          />
        </div>
        <div>
          <label className={labelClass}>Max Retries</label>
          <input
            type="number" value={g.max_retries || ''} placeholder="3" min={0} max={10}
            onChange={(e) => update('max_retries', parseInt(e.target.value) || 0)}
            className={inputClass}
          />
        </div>
      </div>
    </div>
  )
}
