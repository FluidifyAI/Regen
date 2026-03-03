import { BarChart2, TrendingUp, Clock, Activity, Users } from 'lucide-react'

const COMING_SOON_METRICS = [
  {
    icon: BarChart2,
    title: 'Incident frequency',
    description: 'Track how many incidents occur over time, broken down by severity and source.',
  },
  {
    icon: Clock,
    title: 'MTTD & MTTR',
    description: 'Mean time to detect and mean time to resolve — the core SRE health metrics.',
  },
  {
    icon: TrendingUp,
    title: 'Alert volume trends',
    description: 'See which alert sources are noisiest and track volume over time.',
  },
  {
    icon: Activity,
    title: 'On-call load',
    description: 'Understand incident burden per on-call responder across rotations.',
  },
  {
    icon: Users,
    title: 'Responder activity',
    description: 'Who is acknowledging and resolving incidents, and how fast.',
  },
]

export function AnalyticsPage() {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <h1 className="text-2xl font-semibold text-text-primary">Analytics</h1>
        <p className="text-sm text-text-secondary mt-0.5">Incident metrics and on-call insights</p>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {/* Coming soon banner */}
        <div className="max-w-2xl mx-auto text-center py-12">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-blue-50 border border-blue-100 mb-6">
            <BarChart2 className="w-8 h-8 text-brand-primary" />
          </div>
          <h2 className="text-xl font-semibold text-text-primary mb-2">Analytics coming soon</h2>
          <p className="text-text-secondary text-sm max-w-md mx-auto">
            We're building a metrics dashboard so you can understand your incident patterns,
            measure response times, and reduce on-call toil.
          </p>
        </div>

        {/* Preview cards */}
        <div className="max-w-3xl mx-auto grid grid-cols-1 sm:grid-cols-2 gap-4">
          {COMING_SOON_METRICS.map((metric) => {
            const Icon = metric.icon
            return (
              <div
                key={metric.title}
                className="flex gap-4 p-4 rounded-lg border border-border bg-white"
              >
                <div className="flex-shrink-0 w-9 h-9 rounded-lg bg-gray-50 border border-border flex items-center justify-center">
                  <Icon className="w-4.5 h-4.5 text-text-tertiary" />
                </div>
                <div>
                  <div className="text-sm font-medium text-text-primary">{metric.title}</div>
                  <div className="text-xs text-text-secondary mt-0.5 leading-relaxed">{metric.description}</div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
