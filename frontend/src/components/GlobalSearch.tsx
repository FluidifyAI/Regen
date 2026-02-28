import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, X } from 'lucide-react'
import { Badge } from './ui/Badge'
import { listIncidents } from '../api/incidents'
import type { Incident } from '../api/types'

interface GlobalSearchProps {
  isOpen: boolean
  onClose: () => void
}

const MAX_VISIBLE = 8

export function GlobalSearch({ isOpen, onClose }: GlobalSearchProps) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  // Fetch a recent snapshot when the modal opens (lazy, one request per open)
  useEffect(() => {
    if (!isOpen) {
      setQuery('')
      setSelectedIndex(0)
      return
    }
    setLoading(true)
    listIncidents({ limit: 100 })
      .then((res) => setIncidents(res.data ?? []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [isOpen])

  // Focus the input after mount
  useEffect(() => {
    if (!isOpen) return
    const t = setTimeout(() => inputRef.current?.focus(), 50)
    return () => clearTimeout(t)
  }, [isOpen])

  // Filter incidents by query
  const q = query.trim().toLowerCase()
  const filtered = q
    ? incidents.filter(
        (inc) =>
          inc.title.toLowerCase().includes(q) ||
          `inc-${inc.incident_number}`.includes(q),
      )
    : incidents
  const results = filtered.slice(0, MAX_VISIBLE)

  // Reset selection when results change
  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  const openIncident = useCallback(
    (incident: Incident) => {
      navigate(`/incidents/${incident.id}`)
      onClose()
    },
    [navigate, onClose],
  )

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((i) => Math.min(i + 1, results.length - 1))
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((i) => Math.max(i - 1, 0))
      }
      if (e.key === 'Enter') {
        const inc = results[selectedIndex]
        if (inc) openIncident(inc)
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, results, selectedIndex, onClose, openIncident])

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />

      {/* Panel */}
      <div className="relative w-full max-w-xl mx-4 bg-white rounded-xl shadow-2xl overflow-hidden border border-border">
        {/* Search input row */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border">
          <Search className="w-5 h-5 text-text-tertiary flex-shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search incidents by title or number…"
            className="flex-1 text-base text-text-primary placeholder:text-text-tertiary bg-transparent outline-none"
          />
          {query ? (
            <button
              onClick={() => setQuery('')}
              className="text-text-tertiary hover:text-text-primary transition-colors"
              aria-label="Clear search"
            >
              <X className="w-4 h-4" />
            </button>
          ) : (
            <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 border border-border rounded text-xs text-text-tertiary font-mono">
              ESC
            </kbd>
          )}
        </div>

        {/* Results list */}
        {loading ? (
          <div className="px-4 py-8 text-sm text-text-tertiary text-center">
            Loading…
          </div>
        ) : results.length === 0 ? (
          <div className="px-4 py-8 text-sm text-text-tertiary text-center">
            {q ? 'No incidents match your search' : 'No incidents found'}
          </div>
        ) : (
          <ul role="listbox" aria-label="Search results">
            {results.map((inc, i) => (
              <li
                key={inc.id}
                role="option"
                aria-selected={i === selectedIndex}
                onClick={() => openIncident(inc)}
                onMouseEnter={() => setSelectedIndex(i)}
                className={`flex items-center gap-3 px-4 py-3 cursor-pointer transition-colors ${
                  i === selectedIndex ? 'bg-brand-primary/10' : ''
                } ${i < results.length - 1 ? 'border-b border-border' : ''}`}
              >
                <span className="text-xs font-mono text-text-tertiary w-16 flex-shrink-0">
                  INC-{inc.incident_number}
                </span>
                <span className="flex-1 text-sm text-text-primary truncate">
                  {inc.title}
                </span>
                <div className="flex items-center gap-1.5 flex-shrink-0">
                  <Badge variant={inc.severity} type="severity">
                    {inc.severity}
                  </Badge>
                  <Badge variant={inc.status} type="status">
                    {inc.status}
                  </Badge>
                </div>
              </li>
            ))}
          </ul>
        )}

        {/* Footer shortcuts hint */}
        <div className="flex items-center gap-4 px-4 py-2 border-t border-border bg-surface-secondary text-xs text-text-tertiary">
          <span>
            <kbd className="font-mono">↑↓</kbd> navigate
          </span>
          <span>
            <kbd className="font-mono">↵</kbd> open
          </span>
          <span>
            <kbd className="font-mono">ESC</kbd> close
          </span>
          {!loading && (
            <span className="ml-auto">
              {filtered.length} incident{filtered.length !== 1 ? 's' : ''}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
