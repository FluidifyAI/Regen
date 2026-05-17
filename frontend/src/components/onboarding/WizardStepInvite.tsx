import { useState } from 'react'
import { Loader2, AlertCircle, Copy, Check } from 'lucide-react'
import { createUser } from '../../api/settings'

interface Props {
  onComplete: () => void
  onSkip: () => void
}

function generatePassword(): string {
  const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$'
  return Array.from({ length: 16 }, () => chars[Math.floor(Math.random() * chars.length)]).join('')
}

export function WizardStepInvite({ onComplete, onSkip }: Props) {
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState<'member' | 'admin' | 'viewer'>('member')
  const [password] = useState(() => generatePassword())
  const [pwCopied, setPwCopied] = useState(false)
  const [linkCopied, setLinkCopied] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [setupToken, setSetupToken] = useState<string | null>(null)
  const [invitedCount, setInvitedCount] = useState(0)

  const setupLink = setupToken
    ? `${window.location.origin}/login?setup=${setupToken}`
    : null

  async function handleInvite() {
    if (!email || !name) return
    setSaving(true)
    setError('')
    setSetupToken(null)
    try {
      const result = await createUser({ email, name, role, password })
      setSetupToken(result.setup_token)
      setInvitedCount((c) => c + 1)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to invite user'
      setError(msg.includes('409') || msg.toLowerCase().includes('already') ? 'A user with this email already exists.' : msg)
    } finally {
      setSaving(false)
    }
  }

  function handleInviteAnother() {
    setEmail('')
    setName('')
    setRole('member')
    setSetupToken(null)
    setError('')
  }

  function copyPassword() {
    navigator.clipboard.writeText(password)
    setPwCopied(true)
    setTimeout(() => setPwCopied(false), 2000)
  }

  function copyLink() {
    if (setupLink) navigator.clipboard.writeText(setupLink)
    setLinkCopied(true)
    setTimeout(() => setLinkCopied(false), 2000)
  }

  const inputClass = 'w-full h-9 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  return (
    <div className="space-y-4">
      <p className="text-sm text-text-secondary">
        Invite a teammate to Regen. They'll receive a one-time setup link to log in and set their password.
      </p>

      {!setupToken && (
        <>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Name <span className="text-red-500">*</span></label>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="Jane Smith" className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Email <span className="text-red-500">*</span></label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="jane@example.com" className={inputClass} />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Role</label>
            <div className="flex gap-2">
              {(['member', 'admin', 'viewer'] as const).map((r) => (
                <button
                  key={r}
                  onClick={() => setRole(r)}
                  className={`px-3 py-1.5 rounded-lg text-sm border transition-colors capitalize ${role === r ? 'border-brand-primary bg-brand-primary/10 text-brand-primary' : 'border-border text-text-secondary hover:bg-surface-secondary'}`}
                >
                  {r}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Temporary Password (auto-generated)</label>
            <div className="flex items-center gap-2">
              <code className="flex-1 h-9 flex items-center px-3 rounded-lg bg-surface-secondary border border-border text-xs font-mono text-text-secondary truncate">
                {password}
              </code>
              <button onClick={copyPassword} className="flex-shrink-0 p-2 rounded-lg border border-border hover:bg-surface-secondary transition-colors" title="Copy password">
                {pwCopied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4 text-text-tertiary" />}
              </button>
            </div>
            <p className="text-xs text-text-tertiary mt-1">They'll be prompted to change it on first login.</p>
          </div>

          {error && (
            <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2">
              <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0 mt-0.5" />
              <span className="text-sm text-red-700">{error}</span>
            </div>
          )}

          <button
            onClick={handleInvite}
            disabled={!email || !name || saving}
            className="flex items-center gap-2 w-full justify-center h-10 rounded-lg bg-brand-primary hover:bg-brand-primary-hover disabled:opacity-50 text-white text-sm font-medium transition-colors"
          >
            {saving && <Loader2 className="w-4 h-4 animate-spin" />}
            Create Account &amp; Get Setup Link
          </button>
        </>
      )}

      {setupToken && setupLink && (
        <div className="space-y-3">
          <div className="rounded-lg bg-green-50 border border-green-200 px-4 py-3">
            <p className="text-sm font-medium text-green-700 mb-2">Account created for {email}</p>
            <p className="text-xs text-green-600 mb-2">Share this one-time setup link with them:</p>
            <div className="flex items-center gap-2">
              <code className="flex-1 text-xs bg-white border border-green-200 rounded px-2 py-1.5 text-text-primary font-mono truncate">
                {setupLink}
              </code>
              <button onClick={copyLink} className="flex-shrink-0 p-1.5 rounded hover:bg-green-100 transition-colors" title="Copy link">
                {linkCopied ? <Check className="w-3.5 h-3.5 text-green-600" /> : <Copy className="w-3.5 h-3.5 text-green-700" />}
              </button>
            </div>
            <a
              href={`mailto:${email}?subject=You've been invited to Regen&body=Hi ${name},%0A%0AYou've been added to Regen. Click the link below to set up your account:%0A%0A${encodeURIComponent(setupLink)}%0A%0AThis link can only be used once.`}
              className="mt-2 inline-block text-xs text-green-700 underline hover:no-underline"
            >
              Open in email client →
            </a>
          </div>

          <div className="flex items-center justify-between">
            <button onClick={handleInviteAnother} className="text-sm text-brand-primary hover:underline">
              + Invite another
            </button>
            <button onClick={onComplete} className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors">
              Done inviting →
            </button>
          </div>
        </div>
      )}

      {!setupToken && (
        <div className="flex items-center justify-between pt-1">
          {invitedCount > 0 ? (
            <p className="text-xs text-text-tertiary">{invitedCount} invited</p>
          ) : (
            <button onClick={onSkip} className="text-sm text-text-tertiary hover:text-text-secondary transition-colors">
              Skip for now →
            </button>
          )}
          {invitedCount > 0 && (
            <button onClick={onComplete} className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors">
              Done inviting →
            </button>
          )}
        </div>
      )}
    </div>
  )
}
