import { type ReactNode } from 'react'

type SeverityVariant = 'critical' | 'high' | 'medium' | 'low'
type StatusVariant = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

interface BadgeProps {
  variant: SeverityVariant | StatusVariant
  children: ReactNode
  type?: 'severity' | 'status'
}

/**
 * Badge component for severity and status indicators
 * Renders with colored dot/icon + text matching incident.io patterns
 */
export function Badge({ variant, children, type = 'severity' }: BadgeProps) {
  const baseStyles = 'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium'

  if (type === 'severity') {
    const severityStyles = {
      critical: 'bg-red-50 text-red-700',
      high: 'bg-orange-50 text-orange-700',
      medium: 'bg-amber-50 text-amber-700',
      low: 'bg-blue-50 text-blue-700',
    }

    const dotColors = {
      critical: 'bg-severity-critical',
      high: 'bg-severity-high',
      medium: 'bg-severity-medium',
      low: 'bg-severity-low',
    }

    return (
      <span className={`${baseStyles} ${severityStyles[variant as SeverityVariant]}`}>
        <span
          className={`h-2 w-2 rounded-full ${dotColors[variant as SeverityVariant]}`}
          aria-hidden="true"
        />
        {children}
      </span>
    )
  }

  // Status badges
  const statusStyles = {
    triggered: 'bg-red-50 text-red-700',
    acknowledged: 'bg-amber-50 text-amber-700',
    resolved: 'bg-green-50 text-green-700',
    canceled: 'bg-gray-50 text-gray-700',
  }

  const dotColors = {
    triggered: 'bg-status-triggered',
    acknowledged: 'bg-status-acknowledged',
    resolved: 'bg-status-resolved',
    canceled: 'bg-gray-400',
  }

  return (
    <span className={`${baseStyles} ${statusStyles[variant as StatusVariant]}`}>
      <span
        className={`h-2 w-2 rounded-full ${dotColors[variant as StatusVariant]}`}
        aria-hidden="true"
      />
      {children}
    </span>
  )
}
