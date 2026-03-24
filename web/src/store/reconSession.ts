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
export type ChainRunState = 'idle' | 'running' | 'paused' | 'completed'
export type NodeStatus = 'pending' | 'running' | 'completed' | 'failed' | 'blocked'

export interface ReconNode {
  id: string
  toolName: string
  x: number
  y: number
  status: NodeStatus
  phaseName?: string
  scanJobId?: string
  resultType?: string
  resultCount?: number
  targetCount?: number
}

export interface ReconEdge {
  id: string
  from: string
  to: string
}

export interface ReconGraph {
  nodes: ReconNode[]
  edges: ReconEdge[]
  runState: ChainRunState
}

interface ReconSessionStore {
  state: SessionState
  steps: ReconStep[]
  currentStepIndex: number
  selectedIds: Set<string>
  selectedDataType: string | null // what type of data is currently loaded
  graph: ReconGraph

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
  addNode: (node: Omit<ReconNode, 'status'> & { status?: NodeStatus }) => void
  removeNode: (nodeId: string) => void
  connectNodes: (from: string, to: string) => void
  disconnectNodes: (edgeId: string) => void
  moveNode: (nodeId: string, x: number, y: number) => void
  updateNode: (nodeId: string, patch: Partial<ReconNode>) => void
  setChainRunState: (runState: ChainRunState) => void
  startChain: () => void
  pauseChain: () => void
  resumeChain: () => void
  resetChain: () => void
  reset: () => void
}

export const useReconSession = create<ReconSessionStore>((set) => ({
  state: 'idle',
  steps: [],
  currentStepIndex: -1,
  selectedIds: new Set(),
  selectedDataType: null,
  graph: {
    nodes: [],
    edges: [],
    runState: 'idle',
  },

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
  addNode: (node) => set((s) => ({
    graph: {
      ...s.graph,
      nodes: [...s.graph.nodes, { ...node, status: node.status ?? 'pending' }],
    },
  })),
  removeNode: (nodeId) => set((s) => ({
    graph: {
      ...s.graph,
      nodes: s.graph.nodes.filter((n) => n.id !== nodeId),
      edges: s.graph.edges.filter((e) => e.from !== nodeId && e.to !== nodeId),
    },
  })),
  connectNodes: (from, to) => set((s) => {
    if (!from || !to || from === to) return s
    const exists = s.graph.edges.some((e) => e.from === from && e.to === to)
    if (exists) return s
    const edge: ReconEdge = {
      id: `edge-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      from,
      to,
    }
    return {
      graph: {
        ...s.graph,
        edges: [...s.graph.edges, edge],
      },
    }
  }),
  disconnectNodes: (edgeId) => set((s) => ({
    graph: {
      ...s.graph,
      edges: s.graph.edges.filter((e) => e.id !== edgeId),
    },
  })),
  moveNode: (nodeId, x, y) => set((s) => ({
    graph: {
      ...s.graph,
      nodes: s.graph.nodes.map((n) => (n.id === nodeId ? { ...n, x, y } : n)),
    },
  })),
  updateNode: (nodeId, patch) => set((s) => ({
    graph: {
      ...s.graph,
      nodes: s.graph.nodes.map((n) => (n.id === nodeId ? { ...n, ...patch } : n)),
    },
  })),
  setChainRunState: (runState) => set((s) => ({
    graph: { ...s.graph, runState },
  })),
  startChain: () => set((s) => ({
    graph: {
      ...s.graph,
      runState: 'running',
      nodes: s.graph.nodes.map((n) => ({ ...n, status: 'pending', resultCount: 0, targetCount: undefined })),
    },
  })),
  pauseChain: () => set((s) => ({
    graph: { ...s.graph, runState: 'paused' },
  })),
  resumeChain: () => set((s) => ({
    graph: { ...s.graph, runState: 'running' },
  })),
  resetChain: () => set((s) => ({
    graph: {
      ...s.graph,
      runState: 'idle',
      nodes: s.graph.nodes.map((n) => ({ ...n, status: 'pending', scanJobId: undefined, resultCount: 0, targetCount: undefined })),
    },
  })),
  reset: () => set({
    state: 'idle', steps: [], currentStepIndex: -1,
    selectedIds: new Set(), selectedDataType: null,
    graph: { nodes: [], edges: [], runState: 'idle' },
  }),
}))
