import { useState, useEffect } from 'react'
import { X, ExternalLink, Megaphone, Zap, Info } from 'lucide-react'
import { getAnnouncements, type Announcement } from '../api/announcements'
import { useAuth } from '../hooks/useAuth'

const DISMISSED_KEY = 'regen-dismissed-announcements'

function getDismissed(): Set<string> {
  try {
    const raw = localStorage.getItem(DISMISSED_KEY)
    return new Set(raw ? JSON.parse(raw) : [])
  } catch {
    return new Set()
  }
}

function saveDismissed(ids: Set<string>) {
  localStorage.setItem(DISMISSED_KEY, JSON.stringify([...ids]))
}

function isExpired(a: Announcement): boolean {
  if (!a.expires_at) return false
  return new Date(a.expires_at) < new Date()
}

const typeStyles: Record<Announcement['type'], { bg: string; text: string; badge: string }> = {
  release:    { bg: 'bg-blue-950 border-blue-800', text: 'text-blue-100', badge: 'bg-blue-600 text-white' },
  pro_upsell: { bg: 'bg-purple-950 border-purple-800', text: 'text-purple-100', badge: 'bg-purple-600 text-white' },
  info:       { bg: 'bg-zinc-900 border-zinc-700', text: 'text-zinc-100', badge: 'bg-zinc-600 text-white' },
}

const TypeIcon = ({ type }: { type: Announcement['type'] }) => {
  if (type === 'release') return <Megaphone className="w-3.5 h-3.5" />
  if (type === 'pro_upsell') return <Zap className="w-3.5 h-3.5" />
  return <Info className="w-3.5 h-3.5" />
}

const typeLabel: Record<Announcement['type'], string> = {
  release:    'New release',
  pro_upsell: 'Pro',
  info:       'Info',
}

export function AnnouncementBanner() {
  const { user } = useAuth()
  const [announcements, setAnnouncements] = useState<Announcement[]>([])
  const [dismissed, setDismissed] = useState<Set<string>>(getDismissed)

  useEffect(() => {
    if (user?.role !== 'admin') return
    getAnnouncements()
      .then(setAnnouncements)
      .catch(() => {})
  }, [user?.role])

  if (user?.role !== 'admin') return null

  const visible = announcements.filter((a) => !dismissed.has(a.id) && !isExpired(a))
  if (visible.length === 0) return null

  const a = visible[0]!
  const styles = typeStyles[a.type]

  const dismiss = () => {
    const next = new Set(dismissed)
    next.add(a.id)
    setDismissed(next)
    saveDismissed(next)
  }

  return (
    <div className={`${styles.bg} border-b ${styles.text} px-4 py-2 flex items-center gap-3 text-sm`}>
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold flex-shrink-0 ${styles.badge}`}>
        <TypeIcon type={a.type} />
        {typeLabel[a.type]}
      </span>
      <span className="font-medium flex-shrink-0">{a.title}</span>
      {a.body && <span className="opacity-80 truncate">{a.body}</span>}
      {a.cta_url && (
        <a
          href={a.cta_url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 ml-1 underline underline-offset-2 opacity-90 hover:opacity-100 flex-shrink-0"
        >
          {a.cta_label ?? 'Learn more'}
          <ExternalLink className="w-3 h-3" />
        </a>
      )}
      <button
        onClick={dismiss}
        className="ml-auto p-1 rounded hover:bg-white/10 transition-colors flex-shrink-0"
        aria-label="Dismiss"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}
