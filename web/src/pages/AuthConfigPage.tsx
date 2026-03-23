import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api/client'
import { clsx } from 'clsx'
import {
  Plus, Trash2, FlaskConical, Power, KeyRound,
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

export function AuthConfigPage() {
  const { workspaceId } = useParams()
  const [creds, setCreds] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ ...emptyForm })
  const [testResults, setTestResults] = useState<Record<string, any>>({})
  const [testing, setTesting] = useState<string | null>(null)

  const load = () => {
    if (!workspaceId) return
    api.listAuth(workspaceId).then((data) => { setCreds(data); setLoading(false) })
  }

  useEffect(() => { load() }, [workspaceId])

  const handleCreate = async () => {
    if (!workspaceId || !form.name) return
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

  const handleDelete = async (id: string) => {
    if (!workspaceId || !confirm('Delete this credential?')) return
    await api.deleteAuth(workspaceId, id)
    load()
  }

  const handleTest = async (id: string) => {
    if (!workspaceId) return
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
    if (!workspaceId) return
    await api.updateAuth(workspaceId, cred.id, { is_active: !cred.is_active })
    load()
  }

  const typeInfo = AUTH_TYPES.find((t) => t.value === form.auth_type)

  if (loading) return <div className="text-muted font-mono text-sm p-8">Loading auth config...</div>

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-heading tracking-tight">Authentication</h1>
          <p className="text-sm text-muted mt-1">
            Credentials for authenticated crawling &amp; fuzzing
          </p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 px-4 py-2 bg-accent/10 hover:bg-accent/20 text-accent border border-accent/30 rounded-lg text-sm font-medium transition-all"
        >
          <Plus size={16} />
          Add Credential
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="bg-surface border border-accent/20 rounded-lg p-6 mb-6 animate-fade-in">
          {/* Name + Type */}
          <div className="flex gap-3 mb-4">
            <input
              type="text"
              placeholder="credential name (e.g. admin, user1)"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20"
              autoFocus
            />
            <div className="relative">
              <select
                value={form.auth_type}
                onChange={(e) => setForm({ ...form, auth_type: e.target.value as AuthType })}
                className="appearance-none bg-deep border border-border rounded-md px-3 py-2 pr-8 text-sm font-mono text-heading focus:outline-none focus:border-accent/50 cursor-pointer"
              >
                {AUTH_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
              <ChevronDown size={14} className="absolute right-2 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
            </div>
          </div>

          {/* Type description */}
          {typeInfo && (
            <div className="flex items-center gap-2 mb-4 px-1">
              <typeInfo.icon size={13} className="text-accent/60" />
              <span className="text-[11px] font-mono text-muted">{typeInfo.desc}</span>
            </div>
          )}

          {/* Dynamic fields */}
          <div className="space-y-3">
            {(form.auth_type === 'basic' || form.auth_type === 'form') && (
              <div className="flex gap-3">
                <input
                  type="text" placeholder="username"
                  value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })}
                  className="flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
                />
                <input
                  type="password" placeholder="password"
                  value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })}
                  className="flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
                />
              </div>
            )}

            {form.auth_type === 'form' && (
              <>
                <input
                  type="text" placeholder="login URL (e.g. https://target.com/api/login)"
                  value={form.login_url} onChange={(e) => setForm({ ...form, login_url: e.target.value })}
                  className="w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
                />
                <textarea
                  placeholder={'login body template (use {{username}} and {{password}})\ne.g. {"email":"{{username}}","password":"{{password}}"}'}
                  value={form.login_body} onChange={(e) => setForm({ ...form, login_body: e.target.value })}
                  rows={3}
                  className="w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50 resize-none"
                />
              </>
            )}

            {(form.auth_type === 'cookie' || form.auth_type === 'bearer') && (
              <input
                type="text"
                placeholder={form.auth_type === 'cookie' ? 'session=abc123; csrf=xyz789' : 'eyJhbGciOi...'}
                value={form.token} onChange={(e) => setForm({ ...form, token: e.target.value })}
                className="w-full bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
              />
            )}

            {form.auth_type === 'header' && (
              <div className="flex gap-3">
                <input
                  type="text" placeholder="header name (e.g. X-API-Key)"
                  value={form.header_name} onChange={(e) => setForm({ ...form, header_name: e.target.value })}
                  className="w-48 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
                />
                <input
                  type="text" placeholder="header value"
                  value={form.header_value} onChange={(e) => setForm({ ...form, header_value: e.target.value })}
                  className="flex-1 bg-deep border border-border rounded-md px-3 py-2 text-sm font-mono text-heading placeholder:text-muted/50 focus:outline-none focus:border-accent/50"
                />
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex justify-end gap-2 mt-5">
            <button
              onClick={() => { setShowForm(false); setForm({ ...emptyForm }) }}
              className="px-4 py-2 text-sm text-muted hover:text-text transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleCreate}
              disabled={!form.name}
              className="px-5 py-2 bg-accent text-void font-semibold text-sm rounded-md hover:bg-accent-dim transition-colors disabled:opacity-40"
            >
              Save
            </button>
          </div>
        </div>
      )}

      {/* Credential list */}
      {creds.length === 0 && !showForm ? (
        <div className="border border-dashed border-border rounded-lg p-12 text-center">
          <KeyRound size={36} className="mx-auto text-muted/20 mb-4" />
          <p className="text-muted font-mono text-sm">No auth credentials configured</p>
          <p className="text-muted/50 text-xs mt-1">
            Add credentials to enable authenticated crawling with katana, ffuf, and static analysis
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {creds.map((cred, i) => {
            const typeObj = AUTH_TYPES.find((t) => t.value === cred.auth_type)
            const Icon = typeObj?.icon || KeyRound
            const testResult = testResults[cred.id]
            return (
              <div
                key={cred.id}
                className="bg-surface border border-border rounded-lg px-5 py-4 hover:border-border-bright transition-colors animate-fade-in"
                style={{ animationDelay: `${i * 40}ms` }}
              >
                <div className="flex items-center gap-4">
                  {/* Icon */}
                  <div className={clsx(
                    'w-9 h-9 rounded-lg flex items-center justify-center shrink-0',
                    cred.is_active ? 'bg-accent/10 text-accent' : 'bg-muted/10 text-muted/40'
                  )}>
                    <Icon size={18} />
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono font-semibold text-heading text-sm">{cred.name}</span>
                      <span className={clsx(
                        'px-1.5 py-0.5 text-[9px] font-mono uppercase tracking-wider rounded border',
                        cred.is_active
                          ? 'bg-completed/10 text-completed border-completed/20'
                          : 'bg-muted/10 text-muted border-muted/20'
                      )}>
                        {cred.is_active ? 'active' : 'disabled'}
                      </span>
                    </div>
                    <div className="flex items-center gap-3 mt-1">
                      <span className="text-[11px] font-mono text-accent/70">{typeObj?.label || cred.auth_type}</span>
                      {cred.username && (
                        <span className="text-[11px] font-mono text-muted">user: {cred.username}</span>
                      )}
                      {cred.login_url && (
                        <span className="text-[11px] font-mono text-muted truncate max-w-xs">{cred.login_url}</span>
                      )}
                    </div>
                  </div>

                  {/* Test result */}
                  {testResult && (
                    <div className={clsx(
                      'text-[10px] font-mono px-2 py-1 rounded border',
                      testResult.success
                        ? 'bg-completed/10 text-completed border-completed/20'
                        : 'bg-failed/10 text-failed border-failed/20'
                    )}>
                      {testResult.success
                        ? `OK${testResult.cookies_count ? ` · ${testResult.cookies_count} cookies` : ''}`
                        : `FAIL: ${testResult.error?.slice(0, 40)}`
                      }
                    </div>
                  )}

                  {/* Actions */}
                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => handleTest(cred.id)}
                      disabled={testing === cred.id}
                      className="p-2 rounded text-muted hover:text-accent hover:bg-accent/10 transition-colors disabled:opacity-40"
                      title="Test login"
                    >
                      <FlaskConical size={15} className={testing === cred.id ? 'animate-pulse' : ''} />
                    </button>
                    <button
                      onClick={() => handleToggle(cred)}
                      className={clsx(
                        'p-2 rounded transition-colors',
                        cred.is_active
                          ? 'text-completed hover:text-muted hover:bg-muted/10'
                          : 'text-muted/40 hover:text-completed hover:bg-completed/10'
                      )}
                      title={cred.is_active ? 'Disable' : 'Enable'}
                    >
                      <Power size={15} />
                    </button>
                    <button
                      onClick={() => handleDelete(cred.id)}
                      className="p-2 rounded text-muted/40 hover:text-failed hover:bg-failed/10 transition-colors"
                      title="Delete"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
