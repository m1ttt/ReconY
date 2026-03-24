import { create } from 'zustand'

interface WSEvent {
  type: string
  workspace_id: string
  scan_job_id?: string
  phase?: number
  tool_name?: string
  data?: any
  timestamp: string
}

interface IPInfo {
  ip: string
  country: string
  country_code?: string
  city?: string
  is_proxy?: boolean
  is_tor?: boolean
}

type ConnectionStatus = 'connected' | 'disconnected' | 'reconnecting'

interface AppStore {
  events: WSEvent[]
  // Live progress: tool_name → latest result_count
  toolProgress: Record<string, number>
  // Connection status
  connectionStatus: ConnectionStatus
  setConnectionStatus: (status: ConnectionStatus) => void
  // IP Information
  ipInfo: IPInfo | null
  setIPInfo: (info: IPInfo | null) => void
  addEvent: (e: WSEvent) => void
  clearEvents: () => void
}

export const useStore = create<AppStore>((set) => ({
  events: [],
  toolProgress: {},
  connectionStatus: 'disconnected',
  ipInfo: null,
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  setIPInfo: (info) => set({ ipInfo: info }),
  addEvent: (e) => set((s) => {
    const newState: Partial<AppStore> = {
      events: [...s.events.slice(-200), e],
      connectionStatus: 'connected',
    }
    // Track live progress
    if (e.type === 'scan.progress' && e.tool_name && e.data?.result_count != null) {
      newState.toolProgress = { ...s.toolProgress, [e.tool_name]: e.data.result_count }
    }
    // Clear progress when tool completes
    if ((e.type === 'scan.completed' || e.type === 'scan.failed') && e.tool_name) {
      const { [e.tool_name]: _, ...rest } = s.toolProgress
      newState.toolProgress = rest
    }
    return newState
  }),
  clearEvents: () => set({ events: [], toolProgress: {} }),
}))
