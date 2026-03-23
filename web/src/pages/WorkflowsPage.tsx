import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { PHASE_NAMES } from '../types'
import { Workflow, Copy, Lock } from 'lucide-react'

export function WorkflowsPage() {
  const [workflows, setWorkflows] = useState<any[]>([])

  useEffect(() => {
    api.listWorkflows().then(setWorkflows)
  }, [])

  const handleDuplicate = async (name: string) => {
    await api.duplicateWorkflow(name)
    api.listWorkflows().then(setWorkflows)
  }

  return (
    <div className="animate-fade-in">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-heading">Workflows</h1>
        <p className="text-sm text-muted mt-1">Preset and custom scan configurations</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {workflows.map((wf, i) => (
          <div
            key={wf.name + i}
            className="bg-surface border border-border rounded-lg p-5 hover:border-border-bright transition-colors animate-fade-in"
            style={{ animationDelay: `${i * 40}ms` }}
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-2">
                <Workflow size={16} className="text-accent" />
                <h3 className="font-mono font-semibold text-heading">{wf.name}</h3>
              </div>
              <div className="flex gap-1">
                {wf.is_builtin && (
                  <span className="px-1.5 py-0.5 text-[9px] font-mono text-muted bg-raised rounded">
                    <Lock size={8} className="inline mr-0.5" /> builtin
                  </span>
                )}
                <button
                  onClick={() => handleDuplicate(wf.name)}
                  className="p-1 rounded text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                  title="Duplicate as custom"
                >
                  <Copy size={13} />
                </button>
              </div>
            </div>

            <p className="text-xs text-subtle mb-4 leading-relaxed">{wf.description}</p>

            {wf.phase_ids && (
              <div className="flex gap-1.5 flex-wrap">
                {wf.phase_ids.map((p: number) => (
                  <span key={p} className="px-2 py-0.5 text-[10px] font-mono bg-accent/8 text-accent/80 border border-accent/15 rounded">
                    {p}: {PHASE_NAMES[p]?.split(' ')[0]}
                  </span>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
