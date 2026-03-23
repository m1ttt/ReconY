import { Wifi } from 'lucide-react'

interface Props {
  config: any
  onChange: (patch: any) => void
}

const inputClass = 'w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20'
const labelClass = 'text-[11px] font-mono text-muted uppercase tracking-wider'

export function ProxySection({ config, onChange }: Props) {
  const p = config.proxy || {}

  const update = (field: string, value: any) => {
    onChange({ proxy: { ...p, [field]: value } })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <Wifi size={16} className="text-accent" />
        <h2 className="text-sm font-semibold text-heading uppercase tracking-wider">Proxy & Rotation</h2>
      </div>

      <div className="space-y-4">
        <div>
          <label className={labelClass}>Proxy URL</label>
          <input
            type="text" value={p.url || ''} placeholder="socks5://127.0.0.1:9050"
            onChange={(e) => update('url', e.target.value)}
            className={inputClass}
          />
        </div>

        <div className="grid grid-cols-2 gap-5">
          <button
            type="button"
            onClick={() => update('rotation_enabled', !p.rotation_enabled)}
            className="flex items-center gap-3 cursor-pointer group text-left"
          >
            <div className={`w-9 h-5 rounded-full transition-colors relative shrink-0 ${
              p.rotation_enabled ? 'bg-accent' : 'bg-elevated'
            }`}>
              <div className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                p.rotation_enabled ? 'translate-x-4' : 'translate-x-0.5'
              }`} />
            </div>
            <span className="text-sm text-text group-hover:text-heading transition-colors">IP Rotation</span>
          </button>

          <button
            type="button"
            onClick={() => update('mullvad_cli', !p.mullvad_cli)}
            className="flex items-center gap-3 cursor-pointer group text-left"
          >
            <div className={`w-9 h-5 rounded-full transition-colors relative shrink-0 ${
              p.mullvad_cli ? 'bg-accent' : 'bg-elevated'
            }`}>
              <div className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                p.mullvad_cli ? 'translate-x-4' : 'translate-x-0.5'
              }`} />
            </div>
            <span className="text-sm text-text group-hover:text-heading transition-colors">Mullvad CLI</span>
          </button>
        </div>

        {p.rotation_enabled && (
          <div className="grid grid-cols-2 gap-5 animate-fade-in">
            <div>
              <label className={labelClass}>Rotate Every N Requests</label>
              <input
                type="number" value={p.rotate_every_n || ''} placeholder="100" min={1}
                onChange={(e) => update('rotate_every_n', parseInt(e.target.value) || 0)}
                className={inputClass}
              />
            </div>
            <div>
              <label className={labelClass}>Rotate Interval</label>
              <input
                type="text" value={p.rotate_interval || ''} placeholder="5m"
                onChange={(e) => update('rotate_interval', e.target.value)}
                className={inputClass}
              />
            </div>
          </div>
        )}

        {p.mullvad_cli && (
          <div className="animate-fade-in">
            <label className={labelClass}>Mullvad Locations (comma-separated)</label>
            <input
              type="text"
              value={(p.mullvad_locations || []).join(', ')}
              placeholder="us, de, nl, se"
              onChange={(e) => update('mullvad_locations', e.target.value.split(',').map((s: string) => s.trim()).filter(Boolean))}
              className={inputClass}
            />
          </div>
        )}
      </div>
    </div>
  )
}
