import { useState } from 'react'
import { X } from 'lucide-react'
import { updateMe } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

interface Props {
  onClose: () => void
}

export function ProfileModal({ onClose }: Props) {
  const { user, refresh } = useAuth()
  const [name, setName] = useState(user?.name ?? '')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    setSuccess('')
    try {
      const payload: { name?: string; current_password?: string; new_password?: string } = {}
      if (name && name !== user?.name) payload.name = name
      if (newPassword) {
        payload.current_password = currentPassword
        payload.new_password = newPassword
      }
      if (Object.keys(payload).length === 0) {
        setError('No changes to save.')
        setSaving(false)
        return
      }
      await updateMe(payload)
      await refresh?.()
      setSuccess('Profile updated.')
      setCurrentPassword('')
      setNewPassword('')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update profile'
      setError(msg.includes('incorrect') ? 'Current password is incorrect.' : msg)
    } finally {
      setSaving(false)
    }
  }

  const inputCls = 'w-full h-10 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
      <div className="w-full max-w-sm bg-surface-primary rounded-xl border border-border shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h2 className="text-base font-semibold text-text-primary">My profile</h2>
          <button onClick={onClose} className="p-1.5 rounded hover:bg-surface-secondary transition-colors">
            <X className="w-4 h-4 text-text-tertiary" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Email</label>
            <div className="h-10 flex items-center px-3 rounded-lg bg-surface-secondary border border-border text-sm text-text-tertiary">
              {user?.email}
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Display name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Your name"
              className={inputCls}
            />
          </div>

          <div className="border-t border-border pt-4 space-y-3">
              <p className="text-xs font-medium text-text-secondary">Change password</p>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">Current password</label>
                <input
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  placeholder="••••••••"
                  className={inputCls}
                />
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">New password <span className="text-text-tertiary">(min. 8 chars)</span></label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="••••••••"
                  className={inputCls}
                />
              </div>
          </div>

          {error && <p className="text-red-500 text-xs">{error}</p>}
          {success && <p className="text-green-600 text-xs">{success}</p>}

          <div className="flex gap-3 pt-1">
            <button type="button" onClick={onClose} className="flex-1 h-10 rounded-lg border border-border text-text-secondary text-sm font-medium hover:bg-surface-secondary transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={saving} className="flex-1 h-10 rounded-lg bg-brand-primary hover:bg-brand-primary/90 text-white text-sm font-medium transition-colors disabled:opacity-50">
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
