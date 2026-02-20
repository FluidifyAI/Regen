import { useState } from 'react'
import { Plus, Trash2, Check, ChevronDown } from 'lucide-react'
import { Button } from '../ui/Button'
import {
  createActionItem,
  updateActionItem,
  deleteActionItem,
} from '../../api/postmortems'
import type { ActionItem, ActionItemStatus } from '../../api/types'

interface ActionItemsProps {
  incidentId: string
  initialItems: ActionItem[]
}

const STATUS_LABELS: Record<ActionItemStatus, string> = {
  open: 'Open',
  in_progress: 'In Progress',
  closed: 'Closed',
}

const STATUS_COLORS: Record<ActionItemStatus, string> = {
  open: 'bg-blue-100 text-blue-700',
  in_progress: 'bg-amber-100 text-amber-700',
  closed: 'bg-green-100 text-green-700',
}

/**
 * ActionItems displays the action items for a post-mortem with inline add/delete
 * and status toggling.
 */
export function ActionItems({ incidentId, initialItems }: ActionItemsProps) {
  const [items, setItems] = useState<ActionItem[]>(initialItems)
  const [showAddForm, setShowAddForm] = useState(false)
  const [newTitle, setNewTitle] = useState('')
  const [newOwner, setNewOwner] = useState('')
  const [newDueDate, setNewDueDate] = useState('')
  const [adding, setAdding] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [updatingId, setUpdatingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function handleAdd() {
    if (!newTitle.trim()) return
    setAdding(true)
    setError(null)
    try {
      const item = await createActionItem(incidentId, {
        title: newTitle.trim(),
        owner: newOwner.trim() || undefined,
        due_date: newDueDate || undefined,
      })
      setItems((prev) => [...prev, item])
      setNewTitle('')
      setNewOwner('')
      setNewDueDate('')
      setShowAddForm(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add action item')
    } finally {
      setAdding(false)
    }
  }

  async function handleStatusChange(item: ActionItem, newStatus: ActionItemStatus) {
    setUpdatingId(item.id)
    setError(null)
    try {
      const updated = await updateActionItem(incidentId, item.id, { status: newStatus })
      setItems((prev) => prev.map((i) => (i.id === item.id ? updated : i)))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update status')
    } finally {
      setUpdatingId(null)
    }
  }

  async function handleDelete(id: string) {
    setDeletingId(id)
    setError(null)
    try {
      await deleteActionItem(incidentId, id)
      setItems((prev) => prev.filter((i) => i.id !== id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete action item')
    } finally {
      setDeletingId(null)
    }
  }

  const openItems = items.filter((i) => i.status !== 'closed')
  const closedItems = items.filter((i) => i.status === 'closed')

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-surface-secondary border-b border-border">
        <span className="text-sm font-medium text-text-primary">
          Action Items
          {items.length > 0 && (
            <span className="ml-2 text-xs text-text-tertiary">
              ({openItems.length} open)
            </span>
          )}
        </span>
        <button
          onClick={() => setShowAddForm(true)}
          className="inline-flex items-center gap-1 text-xs text-brand-primary hover:text-brand-primary-hover transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
          Add item
        </button>
      </div>

      {/* Error */}
      {error && (
        <div className="px-4 py-2 text-xs text-red-600 bg-red-50 border-b border-red-100">
          {error}
        </div>
      )}

      {/* Add form */}
      {showAddForm && (
        <div className="px-4 py-3 bg-white border-b border-border">
          <div className="space-y-2">
            <input
              autoFocus
              type="text"
              placeholder="Action item title *"
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleAdd()
                if (e.key === 'Escape') setShowAddForm(false)
              }}
              className="w-full px-3 py-1.5 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
            />
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="Owner (optional)"
                value={newOwner}
                onChange={(e) => setNewOwner(e.target.value)}
                className="flex-1 px-3 py-1.5 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
              />
              <input
                type="date"
                value={newDueDate}
                onChange={(e) => setNewDueDate(e.target.value)}
                className="px-3 py-1.5 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
                title="Due date"
              />
            </div>
            <div className="flex gap-2 justify-end">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowAddForm(false)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleAdd}
                loading={adding}
                disabled={!newTitle.trim() || adding}
              >
                Add
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Empty state */}
      {items.length === 0 && !showAddForm && (
        <div className="px-4 py-6 text-center text-sm text-text-tertiary bg-white">
          No action items yet. Add items to track follow-up work.
        </div>
      )}

      {/* Open items */}
      {openItems.length > 0 && (
        <ul className="divide-y divide-border bg-white">
          {openItems.map((item) => (
            <ActionItemRow
              key={item.id}
              item={item}
              updating={updatingId === item.id}
              deleting={deletingId === item.id}
              onStatusChange={handleStatusChange}
              onDelete={handleDelete}
            />
          ))}
        </ul>
      )}

      {/* Closed items (collapsed) */}
      {closedItems.length > 0 && (
        <ClosedItemsSection
          items={closedItems}
          updatingId={updatingId}
          deletingId={deletingId}
          onStatusChange={handleStatusChange}
          onDelete={handleDelete}
        />
      )}
    </div>
  )
}

function ActionItemRow({
  item,
  updating,
  deleting,
  onStatusChange,
  onDelete,
}: {
  item: ActionItem
  updating: boolean
  deleting: boolean
  onStatusChange: (item: ActionItem, status: ActionItemStatus) => void
  onDelete: (id: string) => void
}) {
  const isClosed = item.status === 'closed'

  return (
    <li className={`flex items-start gap-3 px-4 py-3 ${deleting ? 'opacity-50' : ''}`}>
      {/* Quick-close checkbox */}
      <button
        onClick={() => onStatusChange(item, isClosed ? 'open' : 'closed')}
        disabled={updating}
        className={`mt-0.5 w-4 h-4 flex-shrink-0 rounded border transition-colors ${
          isClosed
            ? 'bg-green-500 border-green-500 text-white'
            : 'border-border hover:border-brand-primary'
        }`}
        title={isClosed ? 'Reopen' : 'Mark closed'}
      >
        {isClosed && <Check className="w-3 h-3" />}
      </button>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <p className={`text-sm text-text-primary ${isClosed ? 'line-through text-text-tertiary' : ''}`}>
          {item.title}
        </p>
        <div className="flex items-center gap-3 mt-1">
          {item.owner && (
            <span className="text-xs text-text-tertiary">{item.owner}</span>
          )}
          {item.due_date && (
            <span className="text-xs text-text-tertiary">
              Due {new Date(item.due_date).toLocaleDateString()}
            </span>
          )}
        </div>
      </div>

      {/* Status select */}
      <select
        value={item.status}
        onChange={(e) => onStatusChange(item, e.target.value as ActionItemStatus)}
        disabled={updating}
        className={`text-xs px-2 py-0.5 rounded-full border-0 font-medium focus:outline-none cursor-pointer ${STATUS_COLORS[item.status]}`}
      >
        {Object.entries(STATUS_LABELS).map(([val, label]) => (
          <option key={val} value={val}>
            {label}
          </option>
        ))}
      </select>

      {/* Delete */}
      <button
        onClick={() => onDelete(item.id)}
        disabled={deleting}
        className="p-1 text-text-tertiary hover:text-red-500 transition-colors rounded"
        title="Delete action item"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
    </li>
  )
}

function ClosedItemsSection({
  items,
  updatingId,
  deletingId,
  onStatusChange,
  onDelete,
}: {
  items: ActionItem[]
  updatingId: string | null
  deletingId: string | null
  onStatusChange: (item: ActionItem, status: ActionItemStatus) => void
  onDelete: (id: string) => void
}) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border-t border-border bg-white">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-4 py-2 text-xs text-text-tertiary hover:text-text-secondary transition-colors"
      >
        <ChevronDown
          className={`w-3.5 h-3.5 transition-transform ${expanded ? '' : '-rotate-90'}`}
        />
        {items.length} closed item{items.length !== 1 ? 's' : ''}
      </button>
      {expanded && (
        <ul className="divide-y divide-border">
          {items.map((item) => (
            <ActionItemRow
              key={item.id}
              item={item}
              updating={updatingId === item.id}
              deleting={deletingId === item.id}
              onStatusChange={onStatusChange}
              onDelete={onDelete}
            />
          ))}
        </ul>
      )}
    </div>
  )
}
