import { create } from 'zustand'

export interface ReconStep {
  id: string
  toolName: string
  phaseName: string
  status: 'running' | 'completed' | 'failed'
  resultCount: number
  resultType: string // 'subdomains' | 'ports' | 'urls' | etc.
  timestamp: string
  targetCount?: number // how many items were selected for this step
  scanJobId?: string   // set when scan.started event arrives, used to filter results
}

export type SessionState = 'idle' | 'running' | 'reviewing'

interface ReconSessionStore {
  state: SessionState
  steps: ReconStep[]
  currentStepIndex: number
  selectedIds: Set<string>
  selectedDataType: string | null // what type of data is currently loaded

  setState: (s: SessionState) => void
  addStep: (step: ReconStep) => void
  loadSteps: (steps: ReconStep[]) => void
  updateStep: (id: string, patch: Partial<ReconStep>) => void
  setCurrentStep: (index: number) => void
  setSelectedIds: (ids: Set<string>) => void
  toggleSelected: (id: string) => void
  selectAll: (ids: string[]) => void
  clearSelection: () => void
  setSelectedDataType: (type: string | null) => void
  reset: () => void
}

export const useReconSession = create<ReconSessionStore>((set) => ({
  state: 'idle',
  steps: [],
  currentStepIndex: -1,
  selectedIds: new Set(),
  selectedDataType: null,

  setState: (state) => set({ state }),
  addStep: (step) => set((s) => {
    const steps = [...s.steps, step]
    return { steps, currentStepIndex: steps.length - 1 }
  }),
  loadSteps: (steps) => set({ steps, currentStepIndex: steps.length - 1 }),
  updateStep: (id, patch) => set((s) => ({
    steps: s.steps.map((step) => step.id === id ? { ...step, ...patch } : step),
  })),
  setCurrentStep: (index) => set({ currentStepIndex: index }),
  setSelectedIds: (ids) => set({ selectedIds: ids }),
  toggleSelected: (id) => set((s) => {
    const next = new Set(s.selectedIds)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return { selectedIds: next }
  }),
  selectAll: (ids) => set({ selectedIds: new Set(ids) }),
  clearSelection: () => set({ selectedIds: new Set() }),
  setSelectedDataType: (type) => set({ selectedDataType: type }),
  reset: () => set({
    state: 'idle', steps: [], currentStepIndex: -1,
    selectedIds: new Set(), selectedDataType: null,
  }),
}))
