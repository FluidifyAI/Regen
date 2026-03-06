/**
 * BackgroundAnimation — decorative floating alert/incident cards for auth pages.
 * Pure CSS keyframe animations, no JS timers, no re-renders. aria-hidden.
 *
 * Cards cycle through incident lifecycle states and show AI summaries,
 * giving visitors a live preview of the product while they log in.
 */

const CARDS = [
  // --- Alert firing cards ---
  {
    id: 1,
    type: 'alert',
    severity: 'critical',
    title: 'High CPU Usage',
    detail: 'prod-api-03 · 97% for 5 min',
    status: 'firing',
    style: { top: '8%', left: '4%', animationDelay: '0s', animationDuration: '18s' },
  },
  {
    id: 2,
    type: 'incident',
    severity: 'high',
    title: 'INC-041 · DB Connection Pool',
    detail: 'Acknowledged by Sam Chen',
    status: 'acknowledged',
    style: { top: '22%', right: '3%', animationDelay: '3s', animationDuration: '20s' },
  },
  {
    id: 3,
    type: 'resolved',
    severity: 'resolved',
    title: 'INC-040 · API Error Rate',
    detail: 'Resolved in 14 min',
    status: 'resolved',
    style: { bottom: '28%', left: '2%', animationDelay: '6s', animationDuration: '22s' },
  },
  {
    id: 4,
    type: 'ai',
    severity: 'ai',
    title: 'AI Summary generated',
    detail: 'Root cause: memory leak in worker pool after deploy v2.4.1',
    status: 'ai',
    style: { bottom: '12%', right: '2%', animationDelay: '9s', animationDuration: '19s' },
  },
  {
    id: 5,
    type: 'alert',
    severity: 'warning',
    title: 'Disk Space Warning',
    detail: 'logs-storage-01 · 88% full',
    status: 'firing',
    style: { top: '55%', left: '1%', animationDelay: '12s', animationDuration: '21s' },
  },
  {
    id: 6,
    type: 'oncall',
    severity: 'oncall',
    title: 'On-call shift started',
    detail: 'Taylor Kim is now on call',
    status: 'oncall',
    style: { top: '5%', right: '4%', animationDelay: '5s', animationDuration: '17s' },
  },
  {
    id: 7,
    type: 'incident',
    severity: 'critical',
    title: 'INC-042 · Payment Gateway',
    detail: 'Commander: Jamie Park',
    status: 'triggered',
    style: { bottom: '5%', left: '5%', animationDelay: '14s', animationDuration: '23s' },
  },
  {
    id: 8,
    type: 'resolved',
    severity: 'resolved',
    title: 'INC-039 · Latency Spike',
    detail: 'Post-mortem draft ready',
    status: 'resolved',
    style: { top: '40%', right: '1%', animationDelay: '2s', animationDuration: '20s' },
  },
]

const severityConfig: Record<string, { dot: string; badge: string; badgeText: string; icon: string }> = {
  critical: {
    dot: '#DC2626',
    badge: 'bg-red-50 border-red-200',
    badgeText: 'text-red-600',
    icon: '●',
  },
  high: {
    dot: '#EA580C',
    badge: 'bg-orange-50 border-orange-200',
    badgeText: 'text-orange-600',
    icon: '●',
  },
  warning: {
    dot: '#F59E0B',
    badge: 'bg-yellow-50 border-yellow-200',
    badgeText: 'text-yellow-700',
    icon: '●',
  },
  resolved: {
    dot: '#16A34A',
    badge: 'bg-green-50 border-green-200',
    badgeText: 'text-green-600',
    icon: '✓',
  },
  ai: {
    dot: '#b02b52',
    badge: 'bg-purple-50 border-purple-200',
    badgeText: 'text-purple-700',
    icon: '✦',
  },
  oncall: {
    dot: '#0284c7',
    badge: 'bg-sky-50 border-sky-200',
    badgeText: 'text-sky-700',
    icon: '◎',
  },
}

const statusLabels: Record<string, string> = {
  firing: 'FIRING',
  triggered: 'TRIGGERED',
  acknowledged: 'ACKNOWLEDGED',
  resolved: 'RESOLVED',
  ai: 'AI',
  oncall: 'ON-CALL',
}

const DEFAULT_CFG = severityConfig['warning']!

export function BackgroundAnimation() {
  return (
    <>
      <style>{`
        @keyframes floatCard {
          0%   { opacity: 0; transform: translateY(16px) scale(0.96); }
          8%   { opacity: 1; transform: translateY(0)   scale(1); }
          80%  { opacity: 1; transform: translateY(-4px) scale(1); }
          92%  { opacity: 0; transform: translateY(-12px) scale(0.97); }
          100% { opacity: 0; transform: translateY(-12px) scale(0.97); }
        }
        .bg-anim-card {
          animation: floatCard linear infinite both;
        }
        @keyframes aiPulse {
          0%, 100% { opacity: 0.7; }
          50% { opacity: 1; }
        }
        .ai-pulse { animation: aiPulse 2s ease-in-out infinite; }
      `}</style>

      <div
        aria-hidden="true"
        className="pointer-events-none fixed inset-0 overflow-hidden"
        style={{ zIndex: 0 }}
      >
        {CARDS.map((card) => {
          const cfg = severityConfig[card.severity] ?? DEFAULT_CFG
          return (
            <div
              key={card.id}
              className="bg-anim-card absolute"
              style={card.style as React.CSSProperties}
            >
              <div
                className={`flex flex-col gap-1 rounded-xl border bg-white/80 backdrop-blur-sm px-3.5 py-2.5 shadow-sm w-52 ${cfg.badge}`}
                style={{ backdropFilter: 'blur(8px)' }}
              >
                {/* Header row */}
                <div className="flex items-center justify-between gap-2">
                  <span
                    className={`text-[10px] font-bold tracking-wider ${cfg.badgeText} ${card.severity === 'ai' ? 'ai-pulse' : ''}`}
                  >
                    {card.severity === 'ai' ? '✦ ' : ''}{statusLabels[card.status]}
                  </span>
                  <span
                    className="w-1.5 h-1.5 rounded-full flex-shrink-0"
                    style={{ backgroundColor: cfg.dot }}
                  />
                </div>
                {/* Title */}
                <p className="text-[11px] font-semibold text-[#1E293B] leading-snug truncate">
                  {card.title}
                </p>
                {/* Detail */}
                <p className="text-[10px] text-[#64748B] leading-snug line-clamp-2">
                  {card.detail}
                </p>
              </div>
            </div>
          )
        })}
      </div>
    </>
  )
}
