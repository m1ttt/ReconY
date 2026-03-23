import { useEffect, useState, useCallback, useRef } from 'react'
import { api } from '../api/client'
import { clsx } from 'clsx'
import { Settings, KeyRound, Wifi, Wrench, FileText, Save, Check } from 'lucide-react'
import { GeneralSection } from '../components/settings/GeneralSection'
import { APIKeysSection } from '../components/settings/APIKeysSection'
import { ProxySection } from '../components/settings/ProxySection'
import { ToolConfigSection } from '../components/settings/ToolConfigSection'
import { WordlistSection } from '../components/settings/WordlistSection'

const tabs = [
  { id: 'general', label: 'General', icon: Settings },
  { id: 'api_keys', label: 'API Keys', icon: KeyRound },
  { id: 'proxy', label: 'Proxy', icon: Wifi },
  { id: 'tools', label: 'Tools', icon: Wrench },
  { id: 'wordlists', label: 'Wordlists', icon: FileText },
]

export function SettingsPage() {
  const [config, setConfig] = useState<any>(null)
  const [activeTab, setActiveTab] = useState('general')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [dirty, setDirty] = useState(false)
  const saveTimeout = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    api.getConfig().then(setConfig)
  }, [])

  const handleChange = useCallback((patch: any) => {
    setConfig((prev: any) => {
      const next = { ...prev }
      for (const key of Object.keys(patch)) {
        next[key] = typeof patch[key] === 'object' && !Array.isArray(patch[key])
          ? { ...(prev[key] || {}), ...patch[key] }
          : patch[key]
      }
      return next
    })
    setDirty(true)
    setSaved(false)
  }, [])

  const handleSave = async () => {
    if (!config || saving) return
    setSaving(true)
    try {
      const updated = await api.updateConfig(config)
      setConfig(updated)
      setDirty(false)
      setSaved(true)
      if (saveTimeout.current) clearTimeout(saveTimeout.current)
      saveTimeout.current = setTimeout(() => setSaved(false), 2000)
    } catch (e) {
      console.error('Failed to save config:', e)
    }
    setSaving(false)
  }

  if (!config) return <div className="text-muted font-mono text-sm p-8">Loading config...</div>

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-heading tracking-tight">Settings</h1>
          <p className="text-sm text-muted mt-1">Global ReconX configuration</p>
        </div>
        <button
          onClick={handleSave}
          disabled={!dirty || saving}
          className={clsx(
            'flex items-center gap-2 px-5 py-2 rounded-lg text-sm font-medium transition-all',
            saved
              ? 'bg-completed/10 text-completed border border-completed/30'
              : dirty
                ? 'bg-accent text-void hover:bg-accent-dim'
                : 'bg-elevated text-muted border border-border cursor-not-allowed'
          )}
        >
          {saved ? <Check size={15} /> : <Save size={15} />}
          {saving ? 'Saving...' : saved ? 'Saved' : 'Save Changes'}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border mb-6">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => setActiveTab(id)}
            className={clsx(
              'flex items-center gap-2 px-4 py-2.5 text-sm font-medium transition-all border-b-2 -mb-[1px]',
              activeTab === id
                ? 'text-accent border-accent'
                : 'text-muted border-transparent hover:text-text hover:border-border-bright'
            )}
          >
            <Icon size={14} />
            {label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="bg-surface border border-border rounded-lg p-6">
        {activeTab === 'general' && <GeneralSection config={config} onChange={handleChange} />}
        {activeTab === 'api_keys' && <APIKeysSection config={config} onChange={handleChange} />}
        {activeTab === 'proxy' && <ProxySection config={config} onChange={handleChange} />}
        {activeTab === 'tools' && <ToolConfigSection config={config} onChange={handleChange} />}
        {activeTab === 'wordlists' && <WordlistSection config={config} onChange={handleChange} />}
      </div>
    </div>
  )
}
