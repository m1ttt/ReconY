import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { CheckCircle, XCircle } from 'lucide-react'
import type { ToolInfo } from '../types'

export function ToolsPage() {
  const [tools, setTools] = useState<ToolInfo[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.checkTools().then((data) => { setTools(data); setLoading(false) })
  }, [])

  const available = tools.filter((t) => t.available).length

  return (
    <div className="animate-fade-in">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-heading">External Tools</h1>
        <p className="text-sm text-muted mt-1">
          {loading ? 'Checking...' : `${available}/${tools.length} tools available`}
        </p>
      </div>

      <div className="bg-surface border border-border rounded-lg overflow-hidden">
        {tools.map((tool, i) => (
          <div
            key={tool.name}
            className="flex items-center gap-4 px-5 py-3 border-b border-border/50 hover:bg-raised/30 transition-colors animate-fade-in"
            style={{ animationDelay: `${i * 30}ms` }}
          >
            {tool.available ? (
              <CheckCircle size={16} className="text-completed shrink-0" />
            ) : (
              <XCircle size={16} className="text-failed/50 shrink-0" />
            )}
            <span className="font-mono text-sm text-heading w-32">{tool.name}</span>
            <span className="font-mono text-xs text-muted flex-1">{tool.path || tool.error || ''}</span>
            {tool.version && (
              <span className="font-mono text-[10px] text-accent/60 bg-accent/5 px-2 py-0.5 rounded">
                {tool.version.slice(0, 40)}
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
