import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import { Globe, Plus, Trash2, ArrowRight } from 'lucide-react'
import type { Workspace } from '../types'

export function WorkspaceList() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [domain, setDomain] = useState('')
  const [name, setName] = useState('')
  const navigate = useNavigate()

  const load = () => {
    api.listWorkspaces().then((data) => {
      setWorkspaces(data)
      setLoading(false)
    })
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!domain) return
    await api.createWorkspace({ domain, name: name || undefined })
    setDomain('')
    setName('')
    setShowCreate(false)
    load()
  }

  const handleDelete = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation()
    if (!confirm('Delete workspace and all data?')) return
    await api.deleteWorkspace(id)
    load()
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-heading tracking-tight">Workspaces</h1>
          <p className="text-sm text-muted mt-1">Target domains under reconnaissance</p>
        </div>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="flex items-center gap-2 px-4 py-2 bg-accent/10 hover:bg-accent/20 text-accent border border-accent/30 rounded-lg text-sm font-medium transition-all"
        >
          <Plus size={16} />
          New Workspace
        </button>
      </div>

      {/* Create Form */}
      {showCreate && (
        <div className="bg-surface border border-accent/20 rounded-lg p-5 mb-6 animate-fade-in">
          <div className="flex gap-3">
            <input
              type="text"
              placeholder="target domain (e.g. example.com)"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              className="flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20"
              autoFocus
            />
            <input
              type="text"
              placeholder="name (optional)"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              className="w-48 bg-deep border border-border rounded-md px-3 py-2 text-sm text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
            />
            <button
              onClick={handleCreate}
              className="px-5 py-2 bg-accent text-void font-semibold text-sm rounded-md hover:bg-accent-dim transition-colors"
            >
              Create
            </button>
          </div>
        </div>
      )}

      {/* Loading */}
      {loading ? (
        <div className="text-center py-20">
          <span className="font-mono text-muted text-sm">Loading workspaces...</span>
        </div>
      ) : workspaces.length === 0 ? (
        <div className="text-center py-20 border border-dashed border-border rounded-lg">
          <Globe size={40} className="mx-auto text-muted/30 mb-4" />
          <p className="text-muted font-mono text-sm">No workspaces yet</p>
          <p className="text-muted/60 text-xs mt-1">Create one to start recon</p>
        </div>
      ) : (
        /* Workspace Cards */
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {workspaces.map((ws, i) => (
            <div
              key={ws.id}
              onClick={() => navigate(`/workspace/${ws.id}`)}
              className="group bg-surface border border-border rounded-lg p-5 cursor-pointer hover:border-accent/30 hover:bg-raised/30 transition-all animate-fade-in"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              {/* Card Header */}
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="text-heading font-semibold text-base">{ws.name}</h3>
                  <p className="font-mono text-accent text-xs mt-0.5">{ws.domain}</p>
                </div>
                <div className="flex gap-1">
                  <button
                    onClick={(e) => handleDelete(e, ws.id)}
                    className="p-1.5 rounded text-muted/40 hover:text-failed hover:bg-failed/10 transition-colors"
                  >
                    <Trash2 size={14} />
                  </button>
                  <div className="p-1.5 rounded text-muted/40 group-hover:text-accent transition-colors">
                    <ArrowRight size={14} />
                  </div>
                </div>
              </div>

              {/* Stats Grid */}
              {ws.stats && (
                <div className="grid grid-cols-4 gap-2">
                  {[
                    { label: 'Subs', value: ws.stats.subdomains },
                    { label: 'Ports', value: ws.stats.open_ports },
                    { label: 'Techs', value: ws.stats.technologies },
                    { label: 'Vulns', value: ws.stats.vulnerabilities, danger: ws.stats.vulnerabilities > 0 },
                  ].map((stat) => (
                    <div key={stat.label} className="text-center">
                      <p className={`font-mono font-bold text-lg ${stat.danger ? 'text-critical' : 'text-heading'}`}>
                        {stat.value}
                      </p>
                      <p className="text-[9px] font-mono text-muted uppercase tracking-wider">{stat.label}</p>
                    </div>
                  ))}
                </div>
              )}

              {/* Footer */}
              <div className="mt-4 pt-3 border-t border-border/50">
                <p className="text-[10px] font-mono text-muted/50">
                  {ws.id.slice(0, 8)} &bull; {ws.created_at?.slice(0, 10)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
