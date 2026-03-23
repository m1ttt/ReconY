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

interface AppStore {
  events: WSEvent[]
  // Live progress: tool_name → latest result_count
  toolProgress: Record<string, number>
  addEvent: (e: WSEvent) => void
  clearEvents: () => void
}

export const useStore = create<AppStore>((set) => ({
  events: [],
  toolProgress: {},
  addEvent: (e) => set((s) => {
    const newState: Partial<AppStore> = {
      events: [...s.events.slice(-200), e],
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
