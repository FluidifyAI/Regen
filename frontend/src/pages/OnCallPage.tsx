import { useLocation, Link } from 'react-router-dom'
import { Zap, Siren, CalendarDays } from 'lucide-react'
import { RoutingRulesPage } from './RoutingRulesPage'
import { EscalationPoliciesPage } from './EscalationPoliciesPage'
import { SchedulesPage } from './SchedulesPage'

// ─── Tab definitions ──────────────────────────────────────────────────────────

const TABS = [
  { id: 'routes',           label: 'Alert Routes',     href: '/on-call',                  icon: Zap         },
  { id: 'escalation-paths', label: 'Escalation Paths', href: '/on-call/escalation-paths', icon: Siren       },
  { id: 'schedules',        label: 'Schedules',        href: '/on-call/schedules',         icon: CalendarDays },
] as const

type TabId = (typeof TABS)[number]['id']

function activeTabFromPath(path: string): TabId {
  if (path.startsWith('/on-call/escalation-paths')) return 'escalation-paths'
  if (path === '/on-call/schedules')               return 'schedules'
  return 'routes'
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function OnCallPage() {
  const location = useLocation()
  const activeTab = activeTabFromPath(location.pathname)

  return (
    <div className="flex flex-col h-full">
      {/* Tab bar header */}
      <div className="flex-shrink-0 border-b border-border bg-surface-primary px-6 pt-4">
        <h1 className="text-2xl font-semibold text-text-primary">On-call</h1>
        <p className="text-sm text-text-tertiary mt-0.5">
          Manage schedules, escalation paths, and alert routing for your team.
        </p>
        <nav className="flex gap-0 mt-3 -mb-px" aria-label="On-call sections">
          {TABS.map((tab) => {
            const Icon = tab.icon
            return (
              <Link
                key={tab.id}
                to={tab.href}
                className={`
                  flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors whitespace-nowrap
                  ${
                    activeTab === tab.id
                      ? 'border-brand-primary text-brand-primary'
                      : 'border-transparent text-text-secondary hover:text-text-primary hover:border-border'
                  }
                `}
              >
                <Icon className="w-3.5 h-3.5" />
                {tab.label}
              </Link>
            )
          })}
        </nav>
      </div>

      {/* Tab content — each component handles its own scrolling */}
      {activeTab === 'routes'           && <RoutingRulesPage />}
      {activeTab === 'escalation-paths' && <EscalationPoliciesPage />}
      {activeTab === 'schedules'        && <SchedulesPage />}
    </div>
  )
}
