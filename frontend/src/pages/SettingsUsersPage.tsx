import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { Button } from '../components/ui/Button'
import {
  listUsers,
  createUser,
  updateUser,
  deactivateUser,
  resetUserPassword,
  UserRecord,
} from '../api/settings'
import { listAgents, setAgentStatus } from '../api/client'
import type { AIAgent } from '../api/types'

export function SettingsUsersPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showInvite, setShowInvite] = useState(false)
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null)
  const [setupInfo, setSetupInfo] = useState<{ token: string; email: string } | null>(null)
  const [agents, setAgents] = useState<AIAgent[]>([])

  useEffect(() => {
    if (currentUser && currentUser.role !== 'admin') {
      navigate('/')
    }
  }, [currentUser, navigate])

  useEffect(() => {
    loadUsers()
  }, [])

  useEffect(() => {
    listAgents().then(setAgents).catch(() => {})
  }, [])

  async function loadUsers() {
    setLoading(true)
    try {
      const data = await listUsers()
      setUsers(data)
      setError('')
    } catch {
      setError('Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  async function handleDeactivate(u: UserRecord) {
    if (!confirm(`Deactivate ${u.name || u.email}? They will no longer be able to sign in.`)) return
    try {
      await deactivateUser(u.id)
      await loadUsers()
    } catch {
      setError('Failed to deactivate user')
    }
  }

  async function handleResetPassword(u: UserRecord) {
    try {
      const { setup_token } = await resetUserPassword(u.id)
      setSetupInfo({ token: setup_token, email: u.email })
    } catch {
      setError('Failed to reset password')
    }
  }

  async function handleToggleAgent(agent: AIAgent) {
    try {
      await setAgentStatus(agent.id, !agent.active)
      setAgents(prev => prev.map(a => a.id === agent.id ? { ...a, active: !a.active } : a))
    } catch {
      setError('Failed to update agent status')
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* Page Header — matches Incidents page pattern */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Users</h1>
            <p className="text-sm text-text-secondary mt-0.5">Manage team members and access</p>
          </div>
          <Button variant="primary" onClick={() => setShowInvite(true)}>
            <Plus className="w-4 h-4" />
            Invite user
          </Button>
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-6">
        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
            {error}
          </div>
        )}

        {setupInfo && (
          <SetupLinkBox token={setupInfo.token} email={setupInfo.email} onClose={() => setSetupInfo(null)} />
        )}

        {/* Table — matches Incidents table style */}
        <div className="bg-white border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Name</th>
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Email</th>
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Role</th>
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Auth</th>
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Last login</th>
                <th className="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-text-tertiary">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-text-tertiary">Loading…</td>
                </tr>
              ) : users.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-text-tertiary">
                    No users yet. Invite your first team member.
                  </td>
                </tr>
              ) : (
                users.map(u => (
                  <tr
                    key={u.id}
                    className={`hover:bg-gray-50 transition-colors ${u.auth_source === 'deactivated' ? 'opacity-50' : ''}`}
                  >
                    <td className="px-4 py-3 font-medium text-text-primary">{u.name || '—'}</td>
                    <td className="px-4 py-3 text-text-secondary">{u.email}</td>
                    <td className="px-4 py-3"><RoleBadge role={u.role} /></td>
                    <td className="px-4 py-3 text-xs uppercase tracking-wide text-text-tertiary font-medium">{u.auth_source}</td>
                    <td className="px-4 py-3 text-text-secondary">
                      {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : '—'}
                    </td>
                    <td className="px-4 py-3">
                      {u.auth_source !== 'deactivated' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => setEditingUser(u)}
                            className="text-xs text-text-secondary hover:text-text-primary px-2 py-1 rounded hover:bg-gray-100 transition-colors"
                          >
                            Edit
                          </button>
                          {u.auth_source === 'local' && (
                            <button
                              onClick={() => handleResetPassword(u)}
                              className="text-xs text-text-secondary hover:text-text-primary px-2 py-1 rounded hover:bg-gray-100 transition-colors"
                            >
                              Reset pw
                            </button>
                          )}
                          <button
                            onClick={() => handleDeactivate(u)}
                            className="text-xs text-red-500 hover:text-red-700 px-2 py-1 rounded hover:bg-red-50 transition-colors"
                          >
                            Deactivate
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* AI Team Members */}
        {agents.length > 0 && (
          <div className="mt-8">
            <h3 className="text-sm font-semibold text-text-primary mb-3">AI Team Members</h3>
            <div className="border border-border rounded-lg overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-gray-50 border-b border-border">
                    <th className="text-left px-4 py-2.5 font-medium text-text-secondary">Agent</th>
                    <th className="text-left px-4 py-2.5 font-medium text-text-secondary">Domain</th>
                    <th className="text-right px-4 py-2.5 font-medium text-text-secondary">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {agents.map(agent => (
                    <tr key={agent.id} className="border-b border-border last:border-0">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-text-primary">{agent.name}</span>
                          <span className="text-xs px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 font-medium">
                            🤖 AI
                          </span>
                        </div>
                        <p className="text-xs text-text-tertiary mt-0.5">{agent.email}</p>
                      </td>
                      <td className="px-4 py-3 text-text-secondary capitalize">
                        {agent.agent_type === 'postmortem' ? 'Post-mortems' : agent.agent_type}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={agent.active}
                          onClick={() => handleToggleAgent(agent)}
                          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                            agent.active ? 'bg-brand-primary' : 'bg-gray-200'
                          }`}
                          title={agent.active ? 'Enabled — click to disable' : 'Disabled — click to enable'}
                        >
                          <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                            agent.active ? 'translate-x-4' : 'translate-x-0.5'
                          }`} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {showInvite && (
        <InviteModal
          onClose={() => setShowInvite(false)}
          onCreated={(token, email) => {
            setSetupInfo({ token, email })
            setShowInvite(false)
            loadUsers()
          }}
        />
      )}

      {editingUser && (
        <EditModal
          user={editingUser}
          onClose={() => setEditingUser(null)}
          onSaved={() => { setEditingUser(null); loadUsers() }}
        />
      )}
    </div>
  )
}

function RoleBadge({ role }: { role: string }) {
  const styles: Record<string, string> = {
    admin: 'bg-purple-100 text-purple-700 ring-1 ring-purple-200',
    member: 'bg-blue-100 text-blue-700 ring-1 ring-blue-200',
    viewer: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200',
  }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${styles[role] ?? styles.member}`}>
      {role}
    </span>
  )
}

function SetupLinkBox({ token, email, onClose }: { token: string; email?: string; onClose: () => void }) {
  const url = `${window.location.origin}/login?setup=${token}`
  const [copied, setCopied] = useState(false)
  function copyLink() {
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  const mailtoHref = email
    ? `mailto:${encodeURIComponent(email)}?subject=${encodeURIComponent("You've been invited to OpenIncident")}&body=${encodeURIComponent(`Hi,\n\nYou've been invited to join OpenIncident.\n\nClick the link below to set up your account:\n${url}\n\nThis link expires in 7 days.\n`)}`
    : undefined
  return (
    <div className="mb-6 p-4 bg-blue-50 rounded-lg border border-blue-200">
      <div className="flex items-center justify-between mb-2">
        <p className="text-sm font-medium text-text-primary">Share this one-time login link</p>
        <button onClick={onClose} className="text-text-tertiary hover:text-text-secondary text-xs">Dismiss</button>
      </div>
      <p className="text-xs text-text-secondary mb-3">The link expires after 7 days. Share it securely.</p>
      <div className="flex gap-2">
        <input
          readOnly value={url}
          className="flex-1 text-xs border border-border rounded-lg px-3 py-2 text-text-primary font-mono bg-white focus:outline-none"
        />
        <button
          onClick={copyLink}
          className="px-4 py-2 text-xs bg-brand-primary hover:bg-brand-primary/90 text-white rounded-lg transition-colors font-medium min-w-[70px]"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
        {mailtoHref && (
          <a
            href={mailtoHref}
            className="px-4 py-2 text-xs border border-border bg-white hover:bg-gray-50 text-text-primary rounded-lg transition-colors font-medium whitespace-nowrap"
          >
            Send email
          </a>
        )}
      </div>
    </div>
  )
}

const inputCls = 'w-full h-10 rounded-lg border border-border text-text-primary text-sm px-3 bg-white focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent'
const primaryBtn = 'flex-1 h-10 rounded-lg bg-brand-primary hover:bg-brand-primary/90 text-white text-sm font-medium transition-colors disabled:opacity-50'
const secondaryBtn = 'flex-1 h-10 rounded-lg border border-border text-text-secondary hover:text-text-primary text-sm font-medium transition-colors bg-white'

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-text-secondary text-xs font-medium mb-1.5">{label}</label>
      {children}
    </div>
  )
}

function ModalOverlay({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="relative w-full max-w-md mx-4 bg-white rounded-xl border border-border p-6 shadow-xl">
        <button onClick={onClose} className="absolute top-4 right-4 text-text-tertiary hover:text-text-secondary text-lg leading-none">×</button>
        {children}
      </div>
    </div>
  )
}

function InviteModal({ onClose, onCreated }: { onClose: () => void; onCreated: (token: string, email: string) => void }) {
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState('member')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password.length < 8) { setError('Password must be at least 8 characters'); return }
    setLoading(true); setError('')
    try {
      const res = await createUser({ email, name, role, password })
      onCreated(res.setup_token, email)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create user'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModalOverlay onClose={onClose}>
      <h2 className="text-lg font-semibold text-text-primary mb-6">Invite user</h2>
      <form onSubmit={handleSubmit} className="space-y-4">
        <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} required className={inputCls} placeholder="Jane Smith" /></Field>
        <Field label="Email"><input type="email" value={email} onChange={e => setEmail(e.target.value)} required className={inputCls} placeholder="jane@company.com" /></Field>
        <Field label="Role">
          <select value={role} onChange={e => setRole(e.target.value)} className={inputCls}>
            <option value="member">Member</option>
            <option value="admin">Admin</option>
            <option value="viewer">Viewer</option>
          </select>
        </Field>
        <Field label="Initial password"><input type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} className={inputCls} placeholder="Min. 8 characters" /></Field>
        {error && <p className="text-red-500 text-xs">{error}</p>}
        <div className="flex gap-3 pt-2">
          <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
          <button type="submit" disabled={loading} className={primaryBtn}>{loading ? 'Creating…' : 'Create user'}</button>
        </div>
      </form>
    </ModalOverlay>
  )
}

function EditModal({ user, onClose, onSaved }: { user: UserRecord; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(user.name)
  const [role, setRole] = useState<string>(user.role)
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password && password.length < 8) { setError('Password must be at least 8 characters'); return }
    setLoading(true); setError('')
    try {
      await updateUser(user.id, {
        ...(name ? { name } : {}),
        ...(user.auth_source !== 'saml' && role ? { role } : {}),
        ...(password ? { password } : {}),
      })
      onSaved()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update user'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModalOverlay onClose={onClose}>
      <h2 className="text-lg font-semibold text-text-primary mb-1">Edit user</h2>
      <p className="text-text-secondary text-sm mb-6">{user.email}</p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} className={inputCls} /></Field>
        {user.auth_source !== 'saml' && (
          <Field label="Role">
            <select value={role} onChange={e => setRole(e.target.value)} className={inputCls}>
              <option value="member">Member</option>
              <option value="admin">Admin</option>
              <option value="viewer">Viewer</option>
            </select>
          </Field>
        )}
        {user.auth_source === 'local' && (
          <Field label="New password (optional)">
            <input type="password" value={password} onChange={e => setPassword(e.target.value)} minLength={8} className={inputCls} placeholder="Leave blank to keep current" />
          </Field>
        )}
        {error && <p className="text-red-500 text-xs">{error}</p>}
        <div className="flex gap-3 pt-2">
          <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
          <button type="submit" disabled={loading} className={primaryBtn}>{loading ? 'Saving…' : 'Save changes'}</button>
        </div>
      </form>
    </ModalOverlay>
  )
}
