import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { IncidentTable } from '../components/incidents/IncidentTable'
import { CreateIncidentModal } from '../components/incidents/CreateIncidentModal'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyIncidentsList } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useIncidents } from '../hooks/useIncidents'
import { Search, Plus, ChevronLeft, ChevronRight } from 'lucide-react'
import type { Incident } from '../api/types'

const PAGE_SIZE = 20

/**
 * Incidents list page with filtering, search, and pagination
 */
export function IncidentsListPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [severityFilter, setSeverityFilter] = useState<string>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const [showCreateModal, setShowCreateModal] = useState(false)

  // Reset to page 1 whenever server-side filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [statusFilter, severityFilter])

  const { incidents, loading, error, total, refetch } = useIncidents({
    status: statusFilter || undefined,
    severity: severityFilter || undefined,
    limit: PAGE_SIZE,
    page: currentPage,
  })

  // Client-side search filters the current page's results
  const filteredIncidents = searchQuery
    ? incidents.filter((inc) =>
        inc.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        `INC-${inc.incident_number}`.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : incidents

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const pageStart = total === 0 ? 0 : (currentPage - 1) * PAGE_SIZE + 1
  const pageEnd = Math.min(currentPage * PAGE_SIZE, total)

  const handleDeclareIncident = () => setShowCreateModal(true)

  const handleIncidentCreated = (incident: Incident) => {
    navigate(`/incidents/${incident.id}`)
  }

  return (
    <div className="flex flex-col h-full">
      <CreateIncidentModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreated={handleIncidentCreated}
      />
      {/* Page Header */}
      <div className="border-b border-border bg-surface-primary px-4 md:px-6 py-4">
        {/* Title row */}
        <div className="flex items-center justify-between mb-3">
          <h1 className="text-xl md:text-2xl font-semibold text-text-primary">Incidents</h1>
          <Button variant="primary" onClick={handleDeclareIncident}>
            <Plus className="w-4 h-4" />
            <span className="hidden sm:inline">Declare incident</span>
            <span className="sm:hidden">New</span>
          </Button>
        </div>

        {/* Search — full width on all sizes */}
        <div className="relative mb-2">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
          <input
            type="text"
            placeholder="Search incidents..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          />
        </div>

        {/* Filters row — wraps on mobile */}
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="flex-1 min-w-[120px] px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          >
            <option value="">All statuses</option>
            <option value="triggered">Triggered</option>
            <option value="acknowledged">Acknowledged</option>
            <option value="resolved">Resolved</option>
            <option value="canceled">Canceled</option>
          </select>

          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="flex-1 min-w-[120px] px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          >
            <option value="">All severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>

          <div className="text-sm text-text-secondary ml-auto">
            {loading ? '...' : searchQuery
              ? `${filteredIncidents.length} result${filteredIncidents.length !== 1 ? 's' : ''}`
              : `${pageStart}–${pageEnd} of ${total}`
            }
          </div>
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-4 md:p-6">
        {loading && <SkeletonTable rows={10} />}

        {!loading && error && (
          <GeneralError message={error} onRetry={refetch} />
        )}

        {!loading && !error && filteredIncidents.length === 0 && (
          <EmptyIncidentsList
            onDeclare={handleDeclareIncident}
            hasFilters={!!(statusFilter || severityFilter || searchQuery)}
          />
        )}

        {!loading && !error && filteredIncidents.length > 0 && (
          <>
            <IncidentTable incidents={filteredIncidents} />

            {/* Pagination */}
            {!searchQuery && totalPages > 1 && (
              <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
                <span className="text-sm text-text-secondary hidden sm:block">
                  Showing {pageStart}–{pageEnd} of {total} incidents
                </span>
                <div className="flex items-center gap-2 w-full sm:w-auto justify-between sm:justify-start">
                  <button
                    onClick={() => setCurrentPage((p) => p - 1)}
                    disabled={currentPage === 1}
                    className="flex items-center gap-1 px-3 py-2 min-h-[44px] text-sm border border-border rounded-lg text-text-secondary hover:text-text-primary hover:bg-surface-secondary transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    <ChevronLeft className="w-4 h-4" />
                    Previous
                  </button>
                  <span className="text-sm text-text-secondary px-2">
                    {currentPage} / {totalPages}
                  </span>
                  <button
                    onClick={() => setCurrentPage((p) => p + 1)}
                    disabled={currentPage >= totalPages}
                    className="flex items-center gap-1 px-3 py-2 min-h-[44px] text-sm border border-border rounded-lg text-text-secondary hover:text-text-primary hover:bg-surface-secondary transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    Next
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
