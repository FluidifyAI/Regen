import { useState } from 'react'
import { Button } from '../components/ui/Button'
import { IncidentTable } from '../components/incidents/IncidentTable'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyIncidentsList } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useIncidents } from '../hooks/useIncidents'
import { Search, Plus } from 'lucide-react'

/**
 * Incidents list page with filtering and search
 * Features: status/severity filters, search, pagination, declare incident button
 */
export function IncidentsListPage() {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [severityFilter, setSeverityFilter] = useState<string>('')
  const [searchQuery, setSearchQuery] = useState('')

  const { incidents, loading, error, total, refetch } = useIncidents({
    status: statusFilter || undefined,
    severity: severityFilter || undefined,
    limit: 50,
  })

  // Filter incidents by search query (client-side)
  const filteredIncidents = searchQuery
    ? incidents.filter((inc) =>
        inc.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        `INC-${inc.incident_number}`.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : incidents

  const handleDeclareIncident = () => {
    // TODO: Open create incident modal
    console.log('Declare incident clicked')
  }

  return (
    <div className="flex flex-col h-full">
      {/* Page Header */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-2xl font-semibold text-text-primary">Incidents</h1>
          <Button variant="primary" onClick={handleDeclareIncident}>
            <Plus className="w-4 h-4" />
            Declare incident
          </Button>
        </div>

        {/* Filters and Search */}
        <div className="flex items-center gap-3">
          {/* Search */}
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
            <input
              type="text"
              placeholder="Search incidents..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
            />
          </div>

          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          >
            <option value="">All statuses</option>
            <option value="triggered">Triggered</option>
            <option value="acknowledged">Acknowledged</option>
            <option value="resolved">Resolved</option>
            <option value="canceled">Canceled</option>
          </select>

          {/* Severity Filter */}
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value)}
            className="px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          >
            <option value="">All severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>

          {/* Results Count */}
          <div className="text-sm text-text-secondary">
            {loading ? '...' : `${filteredIncidents.length} of ${total}`}
          </div>
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading && <SkeletonTable rows={10} />}

        {!loading && error && (
          <GeneralError message={error} onRetry={refetch} />
        )}

        {!loading && !error && filteredIncidents.length === 0 && (
          <EmptyIncidentsList onDeclare={handleDeclareIncident} />
        )}

        {!loading && !error && filteredIncidents.length > 0 && (
          <IncidentTable incidents={filteredIncidents} />
        )}
      </div>
    </div>
  )
}
