import { BarChart2, Clock, TrendingUp, Activity, Users, ArrowUpRight } from 'lucide-react'

const PRO_FEATURES = [
  {
    icon: BarChart2,
    title: 'Incident frequency',
    description: 'Total incidents, daily trend, and severity breakdown over 7, 14, 30, or 90 days.',
  },
  {
    icon: Clock,
    title: 'MTTD & MTTR',
    description: 'Mean time to detect and mean time to resolve — the core SRE health metrics.',
  },
  {
    icon: TrendingUp,
    title: 'Resolution rate',
    description: 'Track what fraction of incidents reach resolved status over any time window.',
  },
  {
    icon: Activity,
    title: 'Daily trend chart',
    description: 'A bar chart of incidents per day so you can spot spikes at a glance.',
  },
  {
    icon: Users,
    title: 'AI cost per user',
    description: 'See which team members are driving AI usage and how much each one costs.',
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
        <div className="max-w-2xl mx-auto">
          {/* Hero */}
          <div className="text-center py-10">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-brand-primary/10 border border-brand-primary/20 mb-6">
              <BarChart2 className="w-8 h-8 text-brand-primary" />
            </div>
            <div className="inline-flex items-center gap-1.5 bg-brand-primary/10 text-brand-primary text-xs font-semibold px-3 py-1 rounded-full mb-4">
              <ArrowUpRight className="w-3 h-3" />
              Regen Pro
            </div>
            <h2 className="text-xl font-semibold text-text-primary mb-2">
              Unlock incident analytics
            </h2>
            <p className="text-text-secondary text-sm max-w-md mx-auto leading-relaxed">
              Regen Pro adds a full analytics dashboard — MTTD, MTTR, severity breakdowns,
              daily trend charts, and AI cost tracking per user.
            </p>
            <a
              href="https://github.com/FluidifyAI/Regen"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 mt-6 px-5 py-2.5 rounded-lg bg-brand-primary text-white text-sm font-medium hover:bg-brand-primary/90 transition-colors"
            >
              Learn about Regen Pro
              <ArrowUpRight className="w-4 h-4" />
            </a>
          </div>

          {/* Feature list */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pb-8">
            {PRO_FEATURES.map((f) => {
              const Icon = f.icon
              return (
                <div
                  key={f.title}
                  className="flex gap-3 p-4 rounded-lg border border-border bg-white"
                >
                  <div className="flex-shrink-0 w-8 h-8 rounded-lg bg-gray-50 border border-border flex items-center justify-center">
                    <Icon className="w-4 h-4 text-text-tertiary" />
                  </div>
                  <div>
                    <div className="text-sm font-medium text-text-primary">{f.title}</div>
                    <div className="text-xs text-text-secondary mt-0.5 leading-relaxed">{f.description}</div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}
