import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { clsx } from 'clsx'
import {
  X, Plus, Trash2, FlaskConical, Power, KeyRound,
  User, Cookie, Shield, Braces, ChevronDown
} from 'lucide-react'

type AuthType = 'basic' | 'form' | 'cookie' | 'bearer' | 'header'

const AUTH_TYPES: { value: AuthType; label: string; icon: any; desc: string }[] = [
  { value: 'basic', label: 'Basic Auth', icon: User, desc: 'HTTP Basic username:password' },
  { value: 'form', label: 'Form Login', icon: Braces, desc: 'POST to login endpoint' },
  { value: 'cookie', label: 'Cookie', icon: Cookie, desc: 'Raw cookie string' },
  { value: 'bearer', label: 'Bearer Token', icon: Shield, desc: 'Authorization: Bearer' },
  { value: 'header', label: 'Custom Header', icon: KeyRound, desc: 'Custom header injection' },
]

const emptyForm = {
  name: '', auth_type: 'basic' as AuthType,
  username: '', password: '', login_url: '', login_body: '',
  token: '', header_name: '', header_value: '',
}

const inputClass = 'flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50'

interface Props {
  workspaceId: string
  onClose: () => void
}

export function AuthModal({ workspaceId, onClose }: Props) {
  const [creds, setCreds] = useState<any[]>([])
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [testing, setTesting] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, any>>({})

  const load = () => {
    api.listAuth(workspaceId).then(setCreds)
  }

  useEffect(() => { load() }, [workspaceId])

  const handleCreate = async () => {
    if (!form.name) return
    const payload: any = { name: form.name, auth_type: form.auth_type }
    if (form.username) payload.username = form.username
    if (form.password) payload.password = form.password
    if (form.login_url) payload.login_url = form.login_url
    if (form.login_body) payload.login_body = form.login_body
    if (form.token) payload.token = form.token
    if (form.header_name) payload.header_name = form.header_name
    if (form.header_value) payload.header_value = form.header_value
    await api.createAuth(workspaceId, payload)
    setForm({ ...emptyForm })
    setShowForm(false)
    load()
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      const result = await api.testAuth(workspaceId, id)
      setTestResults((prev) => ({ ...prev, [id]: result }))
    } catch (e: any) {
      setTestResults((prev) => ({ ...prev, [id]: { success: false, error: e.message } }))
    }
    setTesting(null)
  }

  const handleToggle = async (cred: any) => {
    await api.updateAuth(workspaceId, cred.id, { is_active: !cred.is_active })
    load()
  }

  const handleDelete = async (id: string) => {
    await api.deleteAuth(workspaceId, id)
    load()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-void/80 backdrop-blur-sm animate-fade-in">
      <div className="bg-surface border border-border rounded-xl w-full max-w-2xl max-h-[80vh] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div>
            <h2 className="text-lg font-bold text-heading">Authentication Setup</h2>
            <p className="text-xs text-muted mt-0.5">Configure credentials for authenticated scanning</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowForm(!showForm)}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-accent/10 text-accent text-xs font-medium rounded-md border border-accent/20 hover:bg-accent/20 transition-colors"
            >
              <Plus size={13} /> Add
            </button>
            <button onClick={onClose} className="p-2 rounded text-muted hover:text-text hover:bg-raised transition-colors">
              <X size={16} />
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-3">
          {/* Create Form */}
          {showForm && (
            <div className="bg-deep border border-accent/20 rounded-lg p-4 space-y-3 animate-fade-in">
              <div className="flex gap-2">
                <input
                  type="text" placeholder="credential name" value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className={inputClass} autoFocus
                />
                <div className="relative">
                  <select
                    value={form.auth_type}
                    onChange={(e) => setForm({ ...form, auth_type: e.target.value as AuthType })}
                    className="appearance-none bg-deep border border-border rounded-md px-3 py-2 pr-7 text-sm font-mono text-heading focus:outline-none focus:border-accent/50 cursor-pointer"
                  >
                    {AUTH_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
                  </select>
                  <ChevronDown size={12} className="absolute right-2 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
                </div>
              </div>

              {(form.auth_type === 'basic' || form.auth_type === 'form') && (
                <div className="flex gap-2">
                  <input type="text" placeholder="username" value={form.username}
                    onChange={(e) => setForm({ ...form, username: e.target.value })} className={inputClass} />
                  <input type="password" placeholder="password" value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })} className={inputClass} />
                </div>
              )}
              {form.auth_type === 'form' && (
                <>
                  <input type="text" placeholder="login URL" value={form.login_url}
                    onChange={(e) => setForm({ ...form, login_url: e.target.value })} className={`w-full ${inputClass}`} />
                  <textarea placeholder='login body: {"email":"{{username}}","password":"{{password}}"}'
                    value={form.login_body} onChange={(e) => setForm({ ...form, login_body: e.target.value })}
                    rows={2} className={`w-full ${inputClass} resize-none`} />
                </>
              )}
              {(form.auth_type === 'cookie' || form.auth_type === 'bearer') && (
                <input type="text" placeholder={form.auth_type === 'cookie' ? 'session=abc123' : 'eyJhbGci...'}
                  value={form.token} onChange={(e) => setForm({ ...form, token: e.target.value })} className={`w-full ${inputClass}`} />
              )}
              {form.auth_type === 'header' && (
                <div className="flex gap-2">
                  <input type="text" placeholder="X-API-Key" value={form.header_name}
                    onChange={(e) => setForm({ ...form, header_name: e.target.value })} className="w-40 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50" />
                  <input type="text" placeholder="value" value={form.header_value}
                    onChange={(e) => setForm({ ...form, header_value: e.target.value })} className={inputClass} />
                </div>
              )}

              <div className="flex justify-end gap-2 pt-1">
                <button onClick={() => { setShowForm(false); setForm({ ...emptyForm }) }}
                  className="px-3 py-1.5 text-xs text-muted hover:text-text transition-colors">Cancel</button>
                <button onClick={handleCreate} disabled={!form.name}
                  className="px-4 py-1.5 bg-accent text-void text-xs font-semibold rounded-md hover:bg-accent-dim disabled:opacity-40 transition-colors">Save</button>
              </div>
            </div>
          )}

          {/* Credentials List */}
          {creds.length === 0 && !showForm && (
            <div className="text-center py-8">
              <Shield size={28} className="mx-auto text-muted/20 mb-3" />
              <p className="text-sm font-mono text-muted">No credentials yet</p>
            </div>
          )}
          {creds.map((cred) => {
            const typeObj = AUTH_TYPES.find((t) => t.value === cred.auth_type)
            const Icon = typeObj?.icon || KeyRound
            const testResult = testResults[cred.id]
            return (
              <div key={cred.id} className="bg-deep border border-border rounded-lg px-4 py-3 flex items-center gap-3">
                <div className={clsx('w-8 h-8 rounded flex items-center justify-center shrink-0',
                  cred.is_active ? 'bg-accent/10 text-accent' : 'bg-muted/10 text-muted/40')}>
                  <Icon size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm text-heading">{cred.name}</span>
                    <span className={clsx('text-[8px] font-mono uppercase px-1 py-0.5 rounded border',
                      cred.is_active ? 'text-completed border-completed/20 bg-completed/10' : 'text-muted border-muted/20 bg-muted/10')}>
                      {cred.is_active ? 'active' : 'off'}
                    </span>
                  </div>
                  <span className="text-[10px] font-mono text-muted">{typeObj?.label}</span>
                </div>
                {testResult && (
                  <span className={clsx('text-[9px] font-mono px-1.5 py-0.5 rounded border',
                    testResult.success ? 'text-completed border-completed/20 bg-completed/10' : 'text-failed border-failed/20 bg-failed/10')}>
                    {testResult.success ? 'OK' : 'FAIL'}
                  </span>
                )}
                <button onClick={() => handleTest(cred.id)} disabled={testing === cred.id}
                  className="p-1.5 rounded text-muted hover:text-accent hover:bg-accent/10 transition-colors">
                  <FlaskConical size={13} className={testing === cred.id ? 'animate-pulse' : ''} />
                </button>
                <button onClick={() => handleToggle(cred)}
                  className={clsx('p-1.5 rounded transition-colors',
                    cred.is_active ? 'text-completed hover:text-muted' : 'text-muted/40 hover:text-completed')}>
                  <Power size={13} />
                </button>
                <button onClick={() => handleDelete(cred.id)}
                  className="p-1.5 rounded text-muted/40 hover:text-failed hover:bg-failed/10 transition-colors">
                  <Trash2 size={13} />
                </button>
              </div>
            )
          })}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-border flex justify-end">
          <button onClick={onClose}
            className="px-5 py-2 bg-accent text-void text-sm font-semibold rounded-md hover:bg-accent-dim transition-colors">
            Done
          </button>
        </div>
      </div>
    </div>
  )
}
