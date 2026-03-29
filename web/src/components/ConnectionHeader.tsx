import { useEffect, useRef, useState } from 'react'
import { Bot } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api/client'

interface MullvadStatus {
  enabled: boolean
  connected: boolean
  status: string
  country?: string
  city?: string
  ip?: string
  hostname?: string
}

// Country code → flag emoji
const FLAG: Record<string, string> = {
  us: '🇺🇸', de: '🇩🇪', nl: '🇳🇱', se: '🇸🇪',
  ch: '🇨🇭', gb: '🇬🇧', jp: '🇯🇵', fr: '🇫🇷',
  ca: '🇨🇦', au: '🇦🇺', no: '🇳🇴', dk: '🇩🇰',
  fi: '🇫🇮', be: '🇧🇪', at: '🇦🇹', es: '🇪🇸',
  it: '🇮🇹', pt: '🇵🇹', cz: '🇨🇿', ro: '🇷🇴',
  hk: '🇭🇰', sg: '🇸🇬', br: '🇧🇷', mx: '🇲🇽',
}
const flag = (code: string) => FLAG[code.toLowerCase()] ?? '🌐'

const DEFAULT_LOCATIONS = ['us', 'de', 'nl', 'se', 'ch', 'gb', 'jp']

export function ConnectionHeader() {
  const connectionStatus = useStore((s) => s.connectionStatus)
  const ipInfo = useStore((s) => s.ipInfo)
  const setIPInfo = useStore((s) => s.setIPInfo)

  const [isLoadingIP, setIsLoadingIP] = useState(true)
  const [mullvad, setMullvad] = useState<MullvadStatus | null>(null)
  const [locations, setLocations] = useState<string[]>(DEFAULT_LOCATIONS)
  const [rotating, setRotating] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const pickerRef = useRef<HTMLDivElement>(null)
  const lastKnownIPRef = useRef<string | null>(null)

  const isUnknownIP = (ip?: string) => {
    if (!ip) return true
    const normalized = ip.trim().toLowerCase()
    return normalized === '' || normalized === 'unknown' || normalized === 'n/a'
  }

  const applyIPInfo = (data: {
    ip: string
    country: string
    country_code: string
    city?: string
    is_proxy: boolean
    is_tor: boolean
  }) => {
    if (!isUnknownIP(data.ip)) {
      lastKnownIPRef.current = data.ip
      setIPInfo(data)
      setIsLoadingIP(false)
      return true
    }

    // Avoid replacing a known-good IP with a transient "Unknown".
    if (!lastKnownIPRef.current) {
      setIPInfo(data)
    }
    return false
  }

  const refreshIPWithRetry = async (attempts = 12, delayMs = 1_000) => {
    for (let i = 0; i < attempts; i++) {
      try {
        const data = await api.getIpInfo()
        if (applyIPInfo(data)) return
      } catch {
        // keep retrying
      }
      if (i < attempts - 1) {
        await new Promise((resolve) => setTimeout(resolve, delayMs))
      }
    }
    setIsLoadingIP(false)
  }

  // Close picker on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target as Node))
        setPickerOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  // Load config to get mullvad_locations
  useEffect(() => {
    api.getConfig().then((cfg: any) => {
      const locs = cfg?.proxy?.mullvad_locations
      if (Array.isArray(locs) && locs.length > 0) setLocations(locs)
    }).catch(() => null)
  }, [])

  // Fetch IP — retry every 3s until success (backend may not be ready yet),
  // then poll every 30s so it updates after Mullvad rotates.
  useEffect(() => {
    setIsLoadingIP(true)
    void refreshIPWithRetry(20, 3_000)
    const iv = setInterval(() => {
      api.getIpInfo().then(applyIPInfo).catch(() => null)
    }, 30_000)
    return () => clearInterval(iv)
  }, [setIPInfo])

  // Poll Mullvad status every 5s
  useEffect(() => {
    const fetch = () => api.getMullvadStatus().then(setMullvad).catch(() => null)
    fetch()
    const iv = setInterval(fetch, 5_000)
    return () => clearInterval(iv)
  }, [])

  const rotateTo = async (loc: string) => {
    setPickerOpen(false)
    setRotating(true)
    setIsLoadingIP(true)
    try {
      const updated = await api.rotateMullvad(loc)
      setMullvad(updated)
      // Use the IP from the rotate response directly — it's already from the CLI
      // after reconnect --wait, so it's guaranteed to reflect the new exit node.
      if (updated.ip && !isUnknownIP(updated.ip)) {
        lastKnownIPRef.current = updated.ip
        setIPInfo({
          ip: updated.ip,
          country: updated.country ?? '—',
          country_code: '',
          city: updated.city,
          is_proxy: true,
          is_tor: false,
        })
        setIsLoadingIP(false)
      } else {
        void refreshIPWithRetry(15, 1_000)
      }
      // Even when rotate returns IP, refresh a few times to sync geo/IP from /ip-info.
      void refreshIPWithRetry(6, 1_000)
    } catch { /* ignore */ } finally {
      setRotating(false)
    }
  }

  const statusConfig = {
    connected:    { color: 'text-completed', label: 'Connected',      dotColor: 'bg-completed' },
    reconnecting: { color: 'text-medium',    label: 'Reconnecting...', dotColor: 'bg-medium' },
    disconnected: { color: 'text-failed',    label: 'Disconnected',   dotColor: 'bg-failed' },
  }
  const status = statusConfig[connectionStatus]

  // Extract country code from hostname like "ch-zrh-wg-001" → "ch"
  const relayCountry = mullvad?.hostname?.split('-')[0] ?? ''

  return (
    <header className="h-10 border-b border-border flex items-center justify-between px-4 bg-surface text-xs relative z-50">

      {/* Left — ReconX connection status */}
      <div className="flex items-center gap-2">
        <span className={`w-2 h-2 rounded-full ${status.dotColor} ${connectionStatus === 'reconnecting' ? 'animate-pulse' : ''}`} />
        <span className={`font-medium ${status.color}`}>{status.label}</span>
      </div>

      {/* Right — Ask AI button, Mullvad + IP */}
      <div className="flex items-center gap-5">
        <button
          onClick={() => window.dispatchEvent(new CustomEvent('open-ask-ai'))}
          className="flex items-center gap-1.5 px-3 py-1 rounded bg-accent/10 text-accent hover:bg-accent/20 transition-colors border border-accent/20"
        >
          <Bot size={14} /> Ask AI
        </button>

        {/* Mullvad badge + country picker (only when enabled in config) */}
        {mullvad?.enabled && (
          <div className="relative" ref={pickerRef}>
            <button
              onClick={() => !rotating && setPickerOpen(o => !o)}
              className={`flex items-center gap-1.5 px-2 py-0.5 rounded hover:bg-elevated transition-colors ${rotating ? 'opacity-50 cursor-wait' : 'cursor-pointer'}`}
              title="Cambiar país de Mullvad"
            >
              {/* status dot */}
              <span className={`w-1.5 h-1.5 rounded-full ${mullvad.connected ? 'bg-completed animate-pulse' : 'bg-failed'}`} />

              {/* label */}
              <span className={`font-medium ${mullvad.connected ? 'text-completed' : 'text-failed'}`}>
                Mullvad
              </span>

              {/* location info */}
              {mullvad.connected && mullvad.country ? (
                <span className="text-muted">
                  {flag(relayCountry)} {mullvad.country}{mullvad.city ? `, ${mullvad.city}` : ''}
                </span>
              ) : (
                <span className="text-muted">{mullvad.connected ? 'ON' : 'OFF'}</span>
              )}

              {/* chevron / spinner */}
              <span className={`text-muted ml-0.5 text-xs ${rotating ? 'animate-spin' : 'opacity-50'}`}>
                {rotating ? '⟳' : '▾'}
              </span>
            </button>

            {/* Dropdown picker */}
            {pickerOpen && (
              <div className="absolute right-0 top-full mt-1 bg-surface border border-border rounded shadow-xl min-w-28 py-1 z-50">
                {locations.map((loc) => (
                  <button
                    key={loc}
                    onClick={() => rotateTo(loc)}
                    className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-elevated text-left transition-colors"
                  >
                    <span className="text-base leading-none">{flag(loc)}</span>
                    <span className="font-mono text-muted uppercase text-xs">{loc}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {/* IP Address */}
        <div className="flex items-center gap-1.5">
          <span className="text-muted">IP:</span>
          <span className="font-mono text-accent">
            {isLoadingIP ? 'Detecting...' : ipInfo?.ip || 'Unknown'}
          </span>
        </div>

        {/* Geo info (when Mullvad disconnected and geo is available) */}
        {ipInfo && !isLoadingIP && !mullvad?.connected && ipInfo.country && ipInfo.country !== '—' && (
          <div className="flex items-center gap-1.5">
            {ipInfo.country_code && ipInfo.country_code !== 'Local' && (
              <span className="px-1.5 py-0.5 rounded bg-elevated text-muted font-mono">
                {ipInfo.country_code}
              </span>
            )}
            <span className="text-text">{ipInfo.country}</span>
            {ipInfo.city && <span className="text-muted">({ipInfo.city})</span>}
          </div>
        )}
      </div>
    </header>
  )
}
