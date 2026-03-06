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
import { listSlackMembers, listTeamsMembers, SlackMember, TeamsMember } from '../api/users'
import { getSlackOAuthStatus } from '../api/slack'
import { getTeamsConfig } from '../api/teams_config'

export function SettingsUsersPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showInvite, setShowInvite] = useState(false)
  const [showSlackImport, setShowSlackImport] = useState(false)
  const [showTeamsImport, setShowTeamsImport] = useState(false)
  const [teamsConfigured, setTeamsConfigured] = useState(false)
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null)
  const [setupInfo, setSetupInfo] = useState<{ token: string; email: string } | null>(null)
  useEffect(() => {
    if (currentUser && currentUser.role !== 'admin') {
      navigate('/')
    }
  }, [currentUser, navigate])

  useEffect(() => {
    loadUsers()
    getTeamsConfig().then(s => setTeamsConfigured(s.configured)).catch(() => {})
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

  return (
    <div className="flex flex-col h-full">
      {/* Page Header — matches Incidents page pattern */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Users</h1>
            <p className="text-sm text-text-secondary mt-0.5">Manage team members and access</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowSlackImport(true)}
              className="flex items-center gap-1.5 h-9 px-3 rounded-lg border border-border bg-white hover:bg-gray-50 text-sm font-medium text-text-secondary hover:text-text-primary transition-colors"
            >
              <SlackIcon className="w-4 h-4" />
              Import from Slack
            </button>
            {teamsConfigured && (
              <button
                onClick={() => setShowTeamsImport(true)}
                className="flex items-center gap-1.5 h-9 px-3 rounded-lg border border-border bg-white hover:bg-gray-50 text-sm font-medium text-text-secondary hover:text-text-primary transition-colors"
              >
                <TeamsIcon className="w-4 h-4" />
                Import from Teams
              </button>
            )}
            <Button variant="primary" onClick={() => setShowInvite(true)}>
              <Plus className="w-4 h-4" />
              Invite user
            </Button>
          </div>
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
        <div className="bg-surface-primary border border-border rounded-lg overflow-hidden">
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

      {showSlackImport && (
        <SlackImportModal
          onClose={() => setShowSlackImport(false)}
          onImported={() => { setShowSlackImport(false); loadUsers() }}
        />
      )}

      {showTeamsImport && (
        <TeamsImportModal
          onClose={() => setShowTeamsImport(false)}
          onImported={() => { setShowTeamsImport(false); loadUsers() }}
        />
      )}
    </div>
  )
}

function TeamsIcon({ className }: { className?: string }) {
  return (
    <img
      src="https://cdn.simpleicons.org/microsoftteams"
      alt="Microsoft Teams"
      className={className}
      onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
    />
  )
}

function TeamsImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [members, setMembers] = useState<TeamsMember[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loadingMembers, setLoadingMembers] = useState(true)
  const [importing, setImporting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listTeamsMembers()
      .then(data => { setMembers(data.members); setError('') })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load Teams members'))
      .finally(() => setLoadingMembers(false))
  }, [])

  function toggleMember(id: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleImport() {
    setImporting(true); setError('')
    const toImport = members.filter(m => selected.has(m.id) && !m.already_imported)
    try {
      for (const m of toImport) {
        const randomPassword = crypto.randomUUID() + crypto.randomUUID()
        await createUser({
          email: m.email || `${m.id}@teams.local`,
          name: m.name,
          role: 'member',
          password: randomPassword,
          teamsUserId: m.id,
        })
      }
      onImported()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Import failed'
      setError(msg)
      setImporting(false)
    }
  }

  const importableSelected = Array.from(selected).filter(id => {
    const m = members.find(x => x.id === id)
    return m && !m.already_imported
  })

  const secondaryBtn = 'h-9 px-4 rounded-lg border border-border text-sm font-medium text-text-secondary hover:bg-surface-secondary transition-colors'
  const primaryBtn = 'h-9 px-4 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors disabled:opacity-50'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="relative w-full max-w-lg mx-4 bg-white rounded-xl border border-border shadow-xl flex flex-col max-h-[80vh]">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <TeamsIcon className="w-5 h-5" />
            <h2 className="text-lg font-semibold text-text-primary">Import from Teams</h2>
          </div>
          <button onClick={onClose} className="text-text-tertiary hover:text-text-secondary text-lg leading-none">×</button>
        </div>

        <div className="mx-6 mt-4 px-3 py-2 rounded-lg bg-blue-50 border border-blue-200 text-xs text-blue-700">
          Imported users will log in with their <strong>local password</strong> — set a temporary one and ask them to change it. To enable SSO, configure Azure AD in Integrations.
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4">
          {loadingMembers ? (
            <p className="text-center text-text-tertiary py-8">Loading Teams members…</p>
          ) : error ? (
            <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">{error}</div>
          ) : members.length === 0 ? (
            <p className="text-center text-text-tertiary py-8">No Teams members found.</p>
          ) : (
            <ul className="divide-y divide-border">
              {members.map(m => (
                <li
                  key={m.id}
                  className={`flex items-center gap-3 py-3 ${m.already_imported ? 'opacity-50' : ''}`}
                >
                  <input
                    type="checkbox"
                    checked={selected.has(m.id)}
                    onChange={() => toggleMember(m.id)}
                    disabled={m.already_imported}
                    className="h-4 w-4 rounded border-gray-300 text-brand-primary focus:ring-brand-primary"
                  />
                  <div className="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center flex-shrink-0 text-xs font-semibold text-blue-700">
                    {m.name.charAt(0).toUpperCase()}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-text-primary truncate">{m.name}</p>
                    <p className="text-xs text-text-tertiary truncate">{m.email}</p>
                  </div>
                  {m.already_imported && (
                    <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-500 font-medium">Imported</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>

        {!loadingMembers && !error && (
          <div className="px-6 py-4 border-t border-border">
            {error && <p className="text-red-500 text-xs mb-3">{error}</p>}
            <div className="flex gap-3">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button
                type="button"
                onClick={handleImport}
                disabled={importableSelected.length === 0 || importing}
                className={primaryBtn}
              >
                {importing ? 'Importing…' : `Import ${importableSelected.length > 0 ? importableSelected.length + ' selected' : ''}`}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function SlackIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zm1.271 0a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zm0 1.271a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zm10.122 2.521a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zm-1.268 0a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zm-2.523 10.122a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zm0-1.268a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z" />
    </svg>
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
    ? `mailto:${encodeURIComponent(email)}?subject=${encodeURIComponent("You've been invited to Fluidify Alert")}&body=${encodeURIComponent(`Hi,\n\nYou've been invited to join Fluidify Alert.\n\nClick the link below to set up your account:\n${url}\n\nThis link expires in 7 days.\n`)}`
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
  const [slackUserId, setSlackUserId] = useState('')
  const [teamsUserId, setTeamsUserId] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password.length < 8) { setError('Password must be at least 8 characters'); return }
    setLoading(true); setError('')
    try {
      const res = await createUser({
        email, name, role, password,
        slackUserId: slackUserId || undefined,
        teamsUserId: teamsUserId || undefined,
      })
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
        <div>
          <label className="block text-text-secondary text-xs font-medium mb-1.5">
            Slack Member ID <span className="text-gray-400 font-normal">(optional)</span>
          </label>
          <input
            type="text"
            value={slackUserId}
            onChange={e => setSlackUserId(e.target.value)}
            placeholder="e.g. U0AJLLY3678"
            className={inputCls}
          />
          <p className="text-xs text-gray-400 mt-1">Find in Slack: click your profile photo → Copy Member ID</p>
        </div>
        <div>
          <label className="block text-text-secondary text-xs font-medium mb-1.5">
            Teams User ID <span className="text-gray-400 font-normal">(optional)</span>
          </label>
          <input
            type="text"
            value={teamsUserId}
            onChange={e => setTeamsUserId(e.target.value)}
            placeholder="e.g. 29dcb621-b60b-4b3d-aa41-..."
            className={inputCls}
          />
          <p className="text-xs text-gray-400 mt-1">Azure AD Admin Center → Users → select user → Object ID</p>
        </div>
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

function SlackImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [members, setMembers] = useState<SlackMember[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loadingMembers, setLoadingMembers] = useState(true)
  const [slackLoginEnabled, setSlackLoginEnabled] = useState<boolean | null>(null)
  const [importing, setImporting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listSlackMembers()
      .then(data => { setMembers(data.members); setError('') })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load Slack members'))
      .finally(() => setLoadingMembers(false))
    getSlackOAuthStatus()
      .then(r => setSlackLoginEnabled(r.enabled))
      .catch(() => setSlackLoginEnabled(false))
  }, [])

  function toggleMember(id: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleImport() {
    setImporting(true); setError('')
    const toImport = members.filter(m => selected.has(m.id))
    try {
      for (const m of toImport) {
        const randomPassword = crypto.randomUUID() + crypto.randomUUID()
        await createUser({
          email: m.email || `${m.id}@slack.local`,
          name: m.name,
          role: 'member',
          password: randomPassword,
          slackUserId: m.id,
        })
      }
      onImported()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Import failed'
      setError(msg)
      setImporting(false)
    }
  }

  const importableSelected = Array.from(selected).filter(id => {
    const m = members.find(x => x.id === id)
    return m && !m.already_imported
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="relative w-full max-w-lg mx-4 bg-white rounded-xl border border-border shadow-xl flex flex-col max-h-[80vh]">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <SlackIcon className="w-5 h-5 text-[#4A154B]" />
            <h2 className="text-lg font-semibold text-text-primary">Import from Slack</h2>
          </div>
          <button onClick={onClose} className="text-text-tertiary hover:text-text-secondary text-lg leading-none">×</button>
        </div>

        {/* Slack login warning */}
        {slackLoginEnabled === false && (
          <div className="mx-6 mt-4 px-3 py-2.5 rounded-lg bg-amber-50 border border-amber-200 text-xs text-amber-800">
            <span className="font-medium">⚠ Slack Login is not configured.</span> Imported users won't be able to sign in until you add OAuth credentials in <strong>Integrations → Slack → Reconfigure</strong>.
          </div>
        )}
        {slackLoginEnabled === true && (
          <div className="mx-6 mt-4 px-3 py-2 rounded-lg bg-blue-50 border border-blue-200 text-xs text-blue-700">
            Imported users will log in with <strong>Continue with Slack</strong> — they won't need a password.
          </div>
        )}

        <div className="flex-1 overflow-y-auto px-6 py-4">
          {loadingMembers ? (
            <p className="text-center text-text-tertiary py-8">Loading Slack members…</p>
          ) : error ? (
            <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">{error}</div>
          ) : members.length === 0 ? (
            <p className="text-center text-text-tertiary py-8">No Slack members found.</p>
          ) : (
            <ul className="divide-y divide-border">
              {members.map(m => (
                <li
                  key={m.id}
                  className={`flex items-center gap-3 py-3 ${m.already_imported ? 'opacity-50' : ''}`}
                >
                  <input
                    type="checkbox"
                    checked={selected.has(m.id)}
                    onChange={() => toggleMember(m.id)}
                    disabled={m.already_imported}
                    className="h-4 w-4 rounded border-gray-300 text-brand-primary focus:ring-brand-primary"
                  />
                  {m.avatar ? (
                    <img src={m.avatar} alt={m.name} className="w-8 h-8 rounded-full flex-shrink-0" />
                  ) : (
                    <div className="w-8 h-8 rounded-full bg-gray-200 flex items-center justify-center flex-shrink-0 text-xs font-medium text-gray-500">
                      {m.name.charAt(0).toUpperCase()}
                    </div>
                  )}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-text-primary truncate">{m.name}</p>
                    <p className="text-xs text-text-tertiary truncate">{m.email}</p>
                  </div>
                  {m.already_imported && (
                    <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-500 font-medium">Imported</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>

        {!loadingMembers && !error && (
          <div className="px-6 py-4 border-t border-border">
            {error && <p className="text-red-500 text-xs mb-3">{error}</p>}
            <div className="flex gap-3">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button
                type="button"
                onClick={handleImport}
                disabled={importableSelected.length === 0 || importing}
                className={primaryBtn}
              >
                {importing ? 'Importing…' : `Import ${importableSelected.length > 0 ? importableSelected.length + ' selected' : ''}`}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function EditModal({ user, onClose, onSaved }: { user: UserRecord; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(user.name)
  const [role, setRole] = useState<string>(user.role)
  const [slackUserId, setSlackUserId] = useState(user.slack_user_id ?? '')
  const [teamsUserId, setTeamsUserId] = useState(user.teams_user_id ?? '')
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
        slackUserId: slackUserId || undefined,
        teamsUserId: teamsUserId || undefined,
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
        <div>
          <label className="block text-text-secondary text-xs font-medium mb-1.5">
            Slack Member ID <span className="text-gray-400 font-normal">(optional)</span>
          </label>
          <input
            type="text"
            value={slackUserId}
            onChange={e => setSlackUserId(e.target.value)}
            placeholder="e.g. U0AJLLY3678"
            className={inputCls}
          />
          <p className="text-xs text-gray-400 mt-1">Find in Slack: click your profile photo → Copy Member ID</p>
        </div>
        <div>
          <label className="block text-text-secondary text-xs font-medium mb-1.5">
            Teams User ID <span className="text-gray-400 font-normal">(optional)</span>
          </label>
          <input
            type="text"
            value={teamsUserId}
            onChange={e => setTeamsUserId(e.target.value)}
            placeholder="e.g. 29dcb621-b60b-4b3d-aa41-..."
            className={inputCls}
          />
          <p className="text-xs text-gray-400 mt-1">Azure AD Admin Center → Users → select user → Object ID</p>
        </div>
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
