/**
 * CommentsThread — flat discussion thread below a post-mortem.
 * Polls every 30s. Anyone can add; anyone can delete any comment (no auth enforcement client-side).
 */
import { useState, useEffect, useCallback, useRef } from 'react'
import { Send, Trash2, MessageSquare } from 'lucide-react'
import {
  listPostMortemComments,
  createPostMortemComment,
  deletePostMortemComment,
} from '../../api/postmortems'
import type { PostMortemComment } from '../../api/types'

interface CommentsThreadProps {
  incidentId: string
}

function initials(name: string): string {
  return name
    .split(' ')
    .map((w) => w[0] ?? '')
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffMins = Math.floor((now.getTime() - d.getTime()) / 60000)
  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  return d.toLocaleDateString()
}

export function CommentsThread({ incidentId }: CommentsThreadProps) {
  const [comments, setComments] = useState<PostMortemComment[]>([])
  const [content, setContent] = useState('')
  const [authorName, setAuthorName] = useState(() => localStorage.getItem('pm_comment_name') ?? '')
  const [submitting, setSubmitting] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  const load = useCallback(async () => {
    try {
      const data = await listPostMortemComments(incidentId)
      setComments(data)
    } catch {
      // silently ignore poll errors
    }
  }, [incidentId])

  useEffect(() => {
    load()
    const interval = setInterval(load, 30_000)
    return () => clearInterval(interval)
  }, [load])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!content.trim() || !authorName.trim()) return
    setSubmitting(true)
    try {
      localStorage.setItem('pm_comment_name', authorName)
      const comment = await createPostMortemComment(incidentId, {
        author_name: authorName.trim(),
        content: content.trim(),
      })
      setComments((prev) => [...prev, comment])
      setContent('')
      setTimeout(() => bottomRef.current?.scrollIntoView({ behavior: 'smooth' }), 50)
    } catch {
      // retry is natural
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: string) {
    setDeletingId(id)
    try {
      await deletePostMortemComment(incidentId, id)
      setComments((prev) => prev.filter((c) => c.id !== id))
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-2.5 bg-surface-secondary border-b border-border">
        <MessageSquare className="w-3.5 h-3.5 text-text-tertiary" />
        <span className="text-xs font-medium text-text-secondary">
          Discussion {comments.length > 0 && `· ${comments.length}`}
        </span>
      </div>

      {/* Comment list */}
      <div className="divide-y divide-border bg-white max-h-80 overflow-y-auto">
        {comments.length === 0 && (
          <p className="text-xs text-text-tertiary text-center py-6">
            No comments yet. Be the first to leave a note.
          </p>
        )}
        {comments.map((c) => (
          <div key={c.id} className="group flex gap-3 px-4 py-3">
            {/* Avatar */}
            <div className="w-7 h-7 rounded-full bg-brand-primary/10 text-brand-primary flex items-center justify-center text-[10px] font-bold flex-shrink-0 mt-0.5">
              {initials(c.author_name)}
            </div>
            {/* Content */}
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2 mb-0.5">
                <span className="text-xs font-semibold text-text-primary">{c.author_name}</span>
                <span className="text-[10px] text-text-tertiary">{formatTime(c.created_at)}</span>
              </div>
              <p className="text-xs text-text-secondary whitespace-pre-wrap break-words">{c.content}</p>
            </div>
            {/* Delete */}
            <button
              onClick={() => handleDelete(c.id)}
              disabled={deletingId === c.id}
              className="opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded text-text-tertiary hover:text-red-500 hover:bg-red-50 flex-shrink-0 self-start"
              title="Delete comment"
            >
              <Trash2 className="w-3 h-3" />
            </button>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <form onSubmit={handleSubmit} className="border-t border-border bg-surface-secondary p-3 space-y-2">
        <input
          type="text"
          value={authorName}
          onChange={(e) => setAuthorName(e.target.value)}
          placeholder="Your name"
          className="w-full h-8 rounded-md bg-white border border-border text-xs text-text-primary px-2.5 placeholder-text-tertiary focus:outline-none focus:ring-1 focus:ring-brand-primary"
        />
        <div className="flex gap-2">
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="Leave a comment…"
            rows={2}
            className="flex-1 rounded-md bg-white border border-border text-xs text-text-primary px-2.5 py-2 placeholder-text-tertiary focus:outline-none focus:ring-1 focus:ring-brand-primary resize-none"
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                handleSubmit(e as unknown as React.FormEvent)
              }
            }}
          />
          <button
            type="submit"
            disabled={submitting || !content.trim() || !authorName.trim()}
            className="self-end p-2 rounded-md bg-brand-primary hover:bg-brand-primary-hover disabled:opacity-40 text-white transition-colors"
            title="Send (⌘↵)"
          >
            <Send className="w-3.5 h-3.5" />
          </button>
        </div>
        <p className="text-[10px] text-text-tertiary">⌘↵ to send</p>
      </form>
    </div>
  )
}
