import { useMemo, useRef, useState, type MouseEvent, type DragEvent } from 'react'
import { clsx } from 'clsx'
import { Play, Pause, RotateCcw, Link2, Trash2 } from 'lucide-react'
import type { ChainRunState, NodeStatus, ReconEdge, ReconNode } from '../store/reconSession'
import type { ToolRegistryEntry } from '../api/client'

interface Props {
  nodes: ReconNode[]
  edges: ReconEdge[]
  runState: ChainRunState
  tools: ToolRegistryEntry[]
  onAddNode: (toolName: string, x: number, y: number) => string
  onMoveNode: (nodeId: string, x: number, y: number) => void
  onRemoveNode: (nodeId: string) => void
  onConnectNodes: (from: string, to: string) => void
  onDisconnectEdge: (edgeId: string) => void
  onStart: () => void
  onPause: () => void
  onResume: () => void
  onReset: () => void
  connectionHint?: string | null
  getEdgeClass?: (fromNodeId: string, toNodeId: string) => 'direct' | 'domain-fallback' | 'incompatible'
  getToolCompatibility?: (fromNodeId: string, toToolName: string) => 'direct' | 'domain-fallback' | 'incompatible'
  onAddToolAndLink?: (fromNodeId: string, toToolName: string) => void
}

const statusClass: Record<NodeStatus, string> = {
  pending: 'border-border text-muted bg-surface',
  running: 'border-running/40 text-running bg-running/10',
  completed: 'border-completed/40 text-completed bg-completed/10',
  failed: 'border-failed/40 text-failed bg-failed/10',
  blocked: 'border-medium/40 text-medium bg-medium/10',
}

export function ChainBuilder({
  nodes,
  edges,
  runState,
  tools,
  onAddNode,
  onMoveNode,
  onRemoveNode,
  onConnectNodes,
  onDisconnectEdge,
  onStart,
  onPause,
  onResume,
  onReset,
  connectionHint,
  getEdgeClass,
  getToolCompatibility,
  onAddToolAndLink,
}: Props) {
  const canvasRef = useRef<HTMLDivElement>(null)
  const [draggingNode, setDraggingNode] = useState<string | null>(null)
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 })
  const [selectedSourceNodeId, setSelectedSourceNodeId] = useState<string | null>(null)
  const [linkDragSourceId, setLinkDragSourceId] = useState<string | null>(null)
  const [hoveredTargetNodeId, setHoveredTargetNodeId] = useState<string | null>(null)
  const [pointer, setPointer] = useState<{ x: number; y: number } | null>(null)

  const toolList = useMemo(() => tools.filter((t) => t.available), [tools])
  const nodeMap = useMemo(() => new Map(nodes.map((n) => [n.id, n])), [nodes])
  const suggestedTargets = useMemo(() => {
    if (!selectedSourceNodeId) return []
    return nodes
      .filter((n) => n.id !== selectedSourceNodeId)
      .filter((n) => !edges.some((e) => e.from === selectedSourceNodeId && e.to === n.id))
      .map((n) => {
        const kind = getEdgeClass?.(selectedSourceNodeId, n.id) || 'incompatible'
        return { node: n, kind }
      })
      .filter((x) => x.kind !== 'incompatible')
      .sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'direct' ? -1 : 1))
  }, [selectedSourceNodeId, nodes, edges, getEdgeClass])
  const globalToolSuggestions = useMemo(() => {
    if (!selectedSourceNodeId || !getToolCompatibility) return []
    const sourceTool = nodeMap.get(selectedSourceNodeId)?.toolName
    return toolList
      .filter((t) => t.name !== sourceTool)
      .map((t) => ({ tool: t, kind: getToolCompatibility(selectedSourceNodeId, t.name) }))
      .filter((x) => x.kind !== 'incompatible')
      .sort((a, b) => (a.kind === b.kind ? a.tool.name.localeCompare(b.tool.name) : a.kind === 'direct' ? -1 : 1))
  }, [selectedSourceNodeId, getToolCompatibility, toolList, nodeMap])

  const beginNodeDrag = (e: MouseEvent, node: ReconNode) => {
    e.preventDefault()
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    setDraggingNode(node.id)
    setDragOffset({ x: e.clientX - rect.left, y: e.clientY - rect.top })
  }

  const handleCanvasMouseMove = (e: MouseEvent) => {
    if (!draggingNode || !canvasRef.current) return
    const rect = canvasRef.current.getBoundingClientRect()
    const nextX = Math.max(8, e.clientX - rect.left - dragOffset.x)
    const nextY = Math.max(8, e.clientY - rect.top - dragOffset.y)
    onMoveNode(draggingNode, nextX, nextY)
  }

  const handleCanvasMouseUp = () => {
    if (draggingNode) setDraggingNode(null)
  }

  const handlePaletteDragStart = (e: DragEvent, toolName: string) => {
    e.dataTransfer.setData('application/recon-tool', toolName)
    e.dataTransfer.effectAllowed = 'copy'
  }

  const handleOutputDragStart = (e: DragEvent, nodeId: string) => {
    setSelectedSourceNodeId(nodeId)
    setLinkDragSourceId(nodeId)
    e.dataTransfer.setData('application/recon-edge-source', nodeId)
    e.dataTransfer.effectAllowed = 'link'
  }

  const handleCanvasDrop = (e: DragEvent) => {
    e.preventDefault()
    setLinkDragSourceId(null)
    setHoveredTargetNodeId(null)
    setPointer(null)
    if (!canvasRef.current) return
    const rect = canvasRef.current.getBoundingClientRect()
    const toolName = e.dataTransfer.getData('application/recon-tool')
    if (toolName) {
      onAddNode(toolName, Math.max(8, e.clientX - rect.left - 70), Math.max(8, e.clientY - rect.top - 28))
    }
  }

  const handleTargetDrop = (e: DragEvent, targetNodeId: string) => {
    e.preventDefault()
    setHoveredTargetNodeId(null)
    setLinkDragSourceId(null)
    setPointer(null)
    const sourceNodeId = e.dataTransfer.getData('application/recon-edge-source')
    if (!sourceNodeId || sourceNodeId === targetNodeId) return
    onConnectNodes(sourceNodeId, targetNodeId)
  }

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden mb-3">
      <div className="px-3 py-2 border-b border-border flex items-center justify-between">
        <div>
          <h3 className="text-[11px] font-mono uppercase tracking-wider text-heading">Chain Builder</h3>
          <p className="text-[10px] font-mono text-muted">Drag tools into canvas and link output -&gt; input</p>
          {connectionHint && <p className="text-[10px] font-mono text-medium mt-1">{connectionHint}</p>}
        </div>
        <div className="flex items-center gap-1.5">
          <button
            onClick={runState === 'paused' ? onResume : onStart}
            className="px-2.5 py-1 rounded bg-accent/15 border border-accent/25 text-accent text-[10px] font-mono hover:bg-accent/25"
          >
            <span className="inline-flex items-center gap-1">{runState === 'paused' ? <Play size={11} /> : <Play size={11} />}{runState === 'paused' ? 'Resume' : 'Run Chain'}</span>
          </button>
          <button
            onClick={onPause}
            disabled={runState !== 'running'}
            className="px-2.5 py-1 rounded bg-elevated border border-border text-text text-[10px] font-mono disabled:opacity-40"
          >
            <span className="inline-flex items-center gap-1"><Pause size={11} />Pause</span>
          </button>
          <button
            onClick={onReset}
            className="px-2.5 py-1 rounded bg-elevated border border-border text-text text-[10px] font-mono hover:border-medium/30"
          >
            <span className="inline-flex items-center gap-1"><RotateCcw size={11} />Reset</span>
          </button>
        </div>
      </div>

      <div className="grid grid-cols-[220px_1fr_210px] min-h-[280px]">
        <div className="border-r border-border p-2 space-y-1 overflow-y-auto max-h-[360px]">
          <div className="text-[10px] font-mono text-muted uppercase tracking-wider px-1">Tool Catalog</div>
          {toolList.map((tool) => (
            <div
              key={tool.name}
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, tool.name)}
              className="px-2 py-1.5 rounded border border-border hover:border-accent/40 hover:bg-raised/50 cursor-grab active:cursor-grabbing"
              title="Drag to canvas"
            >
              <div className="text-[11px] font-mono text-heading">{tool.name}</div>
              <div className="text-[9px] font-mono text-muted">{tool.phase_name}</div>
            </div>
          ))}
        </div>

        <div
          ref={canvasRef}
          className="relative bg-deep/30 overflow-hidden"
          onMouseMove={handleCanvasMouseMove}
          onMouseUp={handleCanvasMouseUp}
          onMouseLeave={handleCanvasMouseUp}
          onDragOver={(e) => {
            e.preventDefault()
            if (!canvasRef.current) return
            const rect = canvasRef.current.getBoundingClientRect()
            setPointer({ x: e.clientX - rect.left, y: e.clientY - rect.top })
          }}
          onDrop={handleCanvasDrop}
        >
          <svg className="absolute inset-0 pointer-events-none w-full h-full">
          {edges.map((edge) => {
            const from = nodeMap.get(edge.from)
            const to = nodeMap.get(edge.to)
            if (!from || !to) return null
            const edgeClass = getEdgeClass?.(edge.from, edge.to) || 'direct'
            const x1 = from.x + 140
            const y1 = from.y + 28
            const x2 = to.x
            const y2 = to.y + 28
            const c1x = x1 + Math.max(30, (x2 - x1) / 2)
            const c2x = x2 - Math.max(30, (x2 - x1) / 2)
            const d = `M ${x1},${y1} C ${c1x},${y1} ${c2x},${y2} ${x2},${y2}`
            const colorClass =
              edgeClass === 'direct'
                ? 'text-completed/70'
                : edgeClass === 'domain-fallback'
                  ? 'text-medium/70'
                  : 'text-failed/70'
              return (
                <path key={edge.id} d={d} stroke="currentColor" strokeWidth="1.5" fill="none" className={colorClass} />
              )
            })}

            {linkDragSourceId && pointer && (() => {
              const source = nodeMap.get(linkDragSourceId)
              if (!source) return null
              const x1 = source.x + 140
              const y1 = source.y + 28
              const x2 = pointer.x
              const y2 = pointer.y
              const c1x = x1 + Math.max(30, (x2 - x1) / 2)
              const c2x = x2 - Math.max(30, (x2 - x1) / 2)
              const d = `M ${x1},${y1} C ${c1x},${y1} ${c2x},${y2} ${x2},${y2}`
              const previewKind = hoveredTargetNodeId && getEdgeClass
                ? getEdgeClass(linkDragSourceId, hoveredTargetNodeId)
                : 'direct'
              const previewClass = previewKind === 'incompatible'
                ? 'text-failed/60'
                : previewKind === 'domain-fallback'
                  ? 'text-medium/60'
                  : 'text-completed/70'
              return <path d={d} stroke="currentColor" strokeWidth="2" strokeDasharray="5 4" fill="none" className={previewClass} />
            })()}
          </svg>

          {nodes.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center text-[11px] font-mono text-muted/60">
              Drop tools here to build your chain
            </div>
          )}

          {nodes.map((node) => (
            <div
              key={node.id}
              className={clsx(
                'absolute w-[140px] rounded-md border shadow-sm select-none',
                statusClass[node.status],
                selectedSourceNodeId && node.id !== selectedSourceNodeId && getEdgeClass?.(selectedSourceNodeId, node.id) === 'incompatible' && 'opacity-35',
                selectedSourceNodeId && node.id !== selectedSourceNodeId && getEdgeClass?.(selectedSourceNodeId, node.id) === 'direct' && 'ring-1 ring-completed/50 bg-completed/5',
                selectedSourceNodeId && node.id !== selectedSourceNodeId && getEdgeClass?.(selectedSourceNodeId, node.id) === 'domain-fallback' && 'ring-1 ring-medium/50 bg-medium/5',
                hoveredTargetNodeId === node.id && 'ring-2 ring-accent/60'
              )}
              style={{ left: node.x, top: node.y }}
            >
              <div
                className="px-2 py-1 border-b border-border/40 cursor-move flex items-center justify-between"
                onMouseDown={(e) => beginNodeDrag(e, node)}
              >
                <span className="text-[10px] font-mono truncate">{node.toolName}</span>
                <button onClick={() => onRemoveNode(node.id)} className="text-muted hover:text-failed" title="Remove node">
                  <Trash2 size={11} />
                </button>
              </div>
              <div className="px-2 py-1 text-[9px] font-mono text-muted flex items-center justify-between">
                <span>{node.status}</span>
                {typeof node.resultCount === 'number' && node.resultCount > 0 && <span>{node.resultCount}</span>}
              </div>
              <div className="px-1.5 pb-1.5 flex items-center justify-between">
                <div
                  className="w-4 h-4 rounded-full border border-border bg-elevated text-muted flex items-center justify-center"
                  onDragOver={(e) => e.preventDefault()}
                  onDragEnter={() => setHoveredTargetNodeId(node.id)}
                  onDragLeave={() => setHoveredTargetNodeId((prev) => (prev === node.id ? null : prev))}
                  onDrop={(e) => handleTargetDrop(e, node.id)}
                  title="Input (drop connection)"
                >
                  <Link2 size={9} />
                </div>
                <div
                  draggable
                  onDragStart={(e) => handleOutputDragStart(e, node.id)}
                  onDragEnd={() => {
                    setLinkDragSourceId(null)
                    setHoveredTargetNodeId(null)
                    setPointer(null)
                  }}
                  onClick={() => setSelectedSourceNodeId(node.id)}
                  className="w-4 h-4 rounded-full border border-accent/40 bg-accent/15 text-accent flex items-center justify-center cursor-crosshair"
                  title="Output (drag to input)"
                >
                  <Link2 size={9} />
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="border-l border-border p-2 space-y-1 overflow-y-auto max-h-[360px]">
          <div className="text-[10px] font-mono text-muted uppercase tracking-wider px-1">Connections</div>
          {selectedSourceNodeId && (
            <div className="px-2 py-1.5 rounded border border-border bg-raised/30 mb-2">
              <div className="text-[10px] font-mono text-muted mb-1">Suggestions from</div>
              <div className="text-[10px] font-mono text-heading truncate mb-1">
                {nodeMap.get(selectedSourceNodeId)?.toolName || selectedSourceNodeId}
              </div>
              {suggestedTargets.length === 0 ? (
                <div className="text-[9px] font-mono text-muted/70">No compatible targets in canvas</div>
              ) : (
                <div className="space-y-1">
                  {suggestedTargets.slice(0, 8).map(({ node, kind }) => (
                    <div key={node.id} className="flex items-center justify-between gap-2">
                      <div className="min-w-0">
                        <div className="text-[10px] font-mono text-text truncate">{node.toolName}</div>
                        <div className={clsx(
                          'text-[9px] font-mono',
                          kind === 'direct' ? 'text-completed' : 'text-medium'
                        )}>
                          {kind === 'direct' ? 'direct' : 'domain fallback'}
                        </div>
                      </div>
                      <button
                        onClick={() => onConnectNodes(selectedSourceNodeId, node.id)}
                        className="text-[9px] font-mono px-1.5 py-0.5 rounded border border-border hover:border-accent/40"
                      >
                        Connect
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <div className="mt-2 pt-2 border-t border-border/60">
                <div className="text-[9px] font-mono text-muted mb-1">Global tools</div>
                {globalToolSuggestions.length === 0 ? (
                  <div className="text-[9px] font-mono text-muted/70">No compatible tools available</div>
                ) : (
                  <div className="space-y-1">
                    {globalToolSuggestions.slice(0, 10).map(({ tool, kind }) => (
                      <div key={tool.name} className="flex items-center justify-between gap-2">
                        <div className="min-w-0">
                          <div className="text-[10px] font-mono text-text truncate">{tool.name}</div>
                          <div className={clsx('text-[9px] font-mono', kind === 'direct' ? 'text-completed' : 'text-medium')}>
                            {kind === 'direct' ? 'direct' : 'domain fallback'}
                          </div>
                        </div>
                        <button
                          onClick={() => selectedSourceNodeId && onAddToolAndLink?.(selectedSourceNodeId, tool.name)}
                          className="text-[9px] font-mono px-1.5 py-0.5 rounded border border-border hover:border-accent/40"
                        >
                          Add + Link
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
          {edges.length === 0 && (
            <p className="text-[10px] font-mono text-muted/60 px-1">No edges yet</p>
          )}
          {edges.map((edge) => {
            const from = nodeMap.get(edge.from)?.toolName || edge.from
            const to = nodeMap.get(edge.to)?.toolName || edge.to
            const edgeClass = getEdgeClass?.(edge.from, edge.to) || 'direct'
            const edgeLabel = edgeClass === 'direct'
              ? 'direct'
              : edgeClass === 'domain-fallback'
                ? 'domain fallback'
                : 'incompatible'
            const edgeLabelClass = edgeClass === 'direct'
              ? 'text-completed'
              : edgeClass === 'domain-fallback'
                ? 'text-medium'
                : 'text-failed'
            return (
              <div key={edge.id} className="px-2 py-1.5 rounded border border-border bg-deep/30">
                <div className="text-[10px] font-mono text-text truncate">{from}{' -> '}{to}</div>
                <div className={`text-[9px] font-mono mt-0.5 ${edgeLabelClass}`}>{edgeLabel}</div>
                <button
                  onClick={() => onDisconnectEdge(edge.id)}
                  className="mt-1 text-[9px] font-mono text-muted hover:text-failed"
                >
                  Remove
                </button>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
