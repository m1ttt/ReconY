import { useState } from 'react'
import { KeyRound, Eye, EyeOff } from 'lucide-react'

interface Props {
  config: any
  onChange: (patch: any) => void
}

const inputClass = 'flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20'
const labelClass = 'text-[11px] font-mono text-muted uppercase tracking-wider'

const keys: { field: string; label: string; placeholder: string }[] = [
  { field: 'shodan', label: 'Shodan API Key', placeholder: 'Enter Shodan API key...' },
  { field: 'censys_id', label: 'Censys API ID', placeholder: 'Enter Censys API ID...' },
  { field: 'censys_secret', label: 'Censys API Secret', placeholder: 'Enter Censys secret...' },
  { field: 'github_token', label: 'GitHub Token', placeholder: 'ghp_...' },
]

export function APIKeysSection({ config, onChange }: Props) {
  const ak = config.api_keys || {}
  const [visible, setVisible] = useState<Record<string, boolean>>({})

  const update = (field: string, value: string) => {
    onChange({ api_keys: { ...ak, [field]: value } })
  }

  const toggle = (field: string) => {
    setVisible((prev) => ({ ...prev, [field]: !prev[field] }))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <KeyRound size={16} className="text-accent" />
        <h2 className="text-sm font-semibold text-heading uppercase tracking-wider">API Keys</h2>
      </div>

      <p className="text-xs text-muted -mt-3">
        Keys are stored locally and masked in the UI. Required for Shodan, Censys, and GitHub dork scanning.
      </p>

      <div className="space-y-4">
        {keys.map(({ field, label, placeholder }) => {
          const value = ak[field] || ''
          const isMasked = value === '***'
          const isVisible = visible[field]

          return (
            <div key={field}>
              <label className={labelClass}>{label}</label>
              <div className="flex items-center gap-2 mt-1">
                <input
                  type={isVisible ? 'text' : 'password'}
                  value={value}
                  placeholder={placeholder}
                  onChange={(e) => update(field, e.target.value)}
                  className={inputClass}
                />
                <button
                  type="button"
                  onClick={() => toggle(field)}
                  className="p-2 rounded text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                  title={isVisible ? 'Hide' : 'Show'}
                >
                  {isVisible ? <EyeOff size={15} /> : <Eye size={15} />}
                </button>
                {isMasked && (
                  <span className="text-[9px] font-mono text-completed/70 bg-completed/10 px-1.5 py-0.5 rounded border border-completed/20">
                    SET
                  </span>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
