import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import {
  listUsers,
  createUser,
  updateUser,
  deactivateUser,
  resetUserPassword,
  UserRecord,
} from '../api/settings'

export function SettingsUsersPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showInvite, setShowInvite] = useState(false)
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null)
  const [setupToken, setSetupToken] = useState('')

  useEffect(() => {
    if (currentUser && currentUser.role !== 'admin') {
      navigate('/')
    }
  }, [currentUser, navigate])

  useEffect(() => {
    loadUsers()
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
      setSetupToken(setup_token)
    } catch {
      setError('Failed to reset password')
    }
  }

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-[#F1F5F9]">Users</h1>
          <p className="text-[#64748B] text-sm mt-1">Manage team members and access</p>
        </div>
        <button
          onClick={() => setShowInvite(true)}
          className="flex items-center gap-2 px-4 py-2 bg-[#2563EB] hover:bg-[#1D4ED8] text-white text-sm font-medium rounded-lg transition-colors"
        >
          + Invite user
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400 text-sm">
          {error}
        </div>
      )}

      {setupToken && (
        <SetupLinkBox token={setupToken} onClose={() => setSetupToken('')} />
      )}

      <div className="rounded-xl border border-[#1E293B] overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-[#1E293B] text-[#64748B] text-xs uppercase tracking-wide">
              <th className="text-left px-4 py-3 font-medium">Name</th>
              <th className="text-left px-4 py-3 font-medium">Email</th>
              <th className="text-left px-4 py-3 font-medium">Role</th>
              <th className="text-left px-4 py-3 font-medium">Auth</th>
              <th className="text-left px-4 py-3 font-medium">Last login</th>
              <th className="text-left px-4 py-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#1E293B]">
            {loading ? (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-[#475569]">Loading…</td>
              </tr>
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-[#475569]">
                  No users yet. Invite your first team member.
                </td>
              </tr>
            ) : (
              users.map(u => (
                <tr
                  key={u.id}
                  className={`bg-[#0F172A] hover:bg-[#1E293B]/50 transition-colors ${
                    u.auth_source === 'deactivated' ? 'opacity-50' : ''
                  }`}
                >
                  <td className="px-4 py-3 text-[#F1F5F9] font-medium">{u.name || '—'}</td>
                  <td className="px-4 py-3 text-[#94A3B8]">{u.email}</td>
                  <td className="px-4 py-3"><RoleBadge role={u.role} /></td>
                  <td className="px-4 py-3 text-[#64748B] text-xs uppercase">{u.auth_source}</td>
                  <td className="px-4 py-3 text-[#64748B]">
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : '—'}
                  </td>
                  <td className="px-4 py-3">
                    {u.auth_source !== 'deactivated' && (
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => setEditingUser(u)}
                          className="text-xs text-[#94A3B8] hover:text-[#F1F5F9] px-2 py-1 rounded hover:bg-[#1E293B]"
                        >
                          Edit
                        </button>
                        {u.auth_source === 'local' && (
                          <button
                            onClick={() => handleResetPassword(u)}
                            className="text-xs text-[#94A3B8] hover:text-[#F1F5F9] px-2 py-1 rounded hover:bg-[#1E293B]"
                          >
                            Reset pw
                          </button>
                        )}
                        <button
                          onClick={() => handleDeactivate(u)}
                          className="text-xs text-red-400 hover:text-red-300 px-2 py-1 rounded hover:bg-red-900/20"
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

      {showInvite && (
        <InviteModal
          onClose={() => setShowInvite(false)}
          onCreated={(token) => {
            setSetupToken(token)
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
  const colors: Record<string, string> = {
    admin: 'bg-purple-500/20 text-purple-300 border-purple-500/30',
    member: 'bg-blue-500/20 text-blue-300 border-blue-500/30',
    viewer: 'bg-gray-500/20 text-gray-300 border-gray-500/30',
  }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${colors[role] ?? colors.member}`}>
      {role}
    </span>
  )
}

function SetupLinkBox({ token, onClose }: { token: string; onClose: () => void }) {
  const url = `${window.location.origin}/login?setup=${token}`
  const [copied, setCopied] = useState(false)
  function copyLink() {
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className="mb-6 p-4 bg-[#1E293B] rounded-xl border border-[#334155]">
      <div className="flex items-center justify-between mb-2">
        <p className="text-sm font-medium text-[#F1F5F9]">Share this one-time login link</p>
        <button onClick={onClose} className="text-[#475569] hover:text-[#94A3B8] text-xs">Dismiss</button>
      </div>
      <p className="text-xs text-[#64748B] mb-3">The link expires after 7 days. Share it securely.</p>
      <div className="flex gap-2">
        <input
          readOnly value={url}
          className="flex-1 text-xs bg-[#0F172A] border border-[#334155] rounded-lg px-3 py-2 text-[#F1F5F9] font-mono"
        />
        <button
          onClick={copyLink}
          className="px-4 py-2 text-xs bg-[#2563EB] hover:bg-[#1D4ED8] text-white rounded-lg transition-colors font-medium min-w-[70px]"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
    </div>
  )
}

const inputCls = 'w-full h-10 rounded-lg bg-[#1E293B] border border-[#334155] text-[#F1F5F9] text-sm px-3 focus:outline-none focus:border-[#2563EB]'
const primaryBtn = 'flex-1 h-10 rounded-lg bg-[#2563EB] hover:bg-[#1D4ED8] text-white text-sm font-medium transition-colors disabled:opacity-50'
const secondaryBtn = 'flex-1 h-10 rounded-lg border border-[#334155] text-[#94A3B8] hover:text-[#F1F5F9] text-sm font-medium transition-colors'

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-[#94A3B8] text-xs font-medium mb-1.5">{label}</label>
      {children}
    </div>
  )
}

function ModalOverlay({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div
        className="relative w-full max-w-md mx-4 bg-[#0F172A] rounded-2xl border border-[#1E293B] p-6"
        style={{ boxShadow: '0 24px 48px rgba(0,0,0,0.4)' }}
      >
        <button onClick={onClose} className="absolute top-4 right-4 text-[#475569] hover:text-[#94A3B8] text-lg leading-none">×</button>
        {children}
      </div>
    </div>
  )
}

function InviteModal({ onClose, onCreated }: { onClose: () => void; onCreated: (token: string) => void }) {
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
      onCreated(res.setup_token)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create user'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModalOverlay onClose={onClose}>
      <h2 className="text-lg font-semibold text-[#F1F5F9] mb-6">Invite user</h2>
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
        {error && <p className="text-red-400 text-xs">{error}</p>}
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
      <h2 className="text-lg font-semibold text-[#F1F5F9] mb-1">Edit user</h2>
      <p className="text-[#64748B] text-sm mb-6">{user.email}</p>
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
        {error && <p className="text-red-400 text-xs">{error}</p>}
        <div className="flex gap-3 pt-2">
          <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
          <button type="submit" disabled={loading} className={primaryBtn}>{loading ? 'Saving…' : 'Save changes'}</button>
        </div>
      </form>
    </ModalOverlay>
  )
}
