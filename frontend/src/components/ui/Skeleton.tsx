/**
 * Skeleton loading components for all major UI patterns
 * Animated pulse effect with #E2E8F0 base and #F1F5F9 highlight
 */

// Base skeleton element
function SkeletonBox({ className = '' }: { className?: string }) {
  return (
    <div
      className={`animate-pulse bg-border rounded ${className}`}
      style={{ backgroundColor: '#E2E8F0' }}
    />
  )
}

/**
 * Skeleton for incidents table on list page
 * Matches column layout: checkbox, incident, severity, status, duration, reported, lead
 */
export function SkeletonTable({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-2">
      {/* Table Header */}
      <div className="grid grid-cols-12 gap-4 px-4 py-3 border-b border-border">
        <SkeletonBox className="col-span-1 h-4 w-4" />
        <SkeletonBox className="col-span-3 h-4" />
        <SkeletonBox className="col-span-2 h-4" />
        <SkeletonBox className="col-span-2 h-4" />
        <SkeletonBox className="col-span-1 h-4" />
        <SkeletonBox className="col-span-2 h-4" />
        <SkeletonBox className="col-span-1 h-4" />
      </div>

      {/* Table Rows */}
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="grid grid-cols-12 gap-4 px-4 py-4 border-b border-border">
          <SkeletonBox className="col-span-1 h-4 w-4" />
          <div className="col-span-3 space-y-2">
            <SkeletonBox className="h-3 w-16" />
            <SkeletonBox className="h-4 w-full" />
          </div>
          <div className="col-span-2 flex items-center gap-2">
            <SkeletonBox className="h-4 w-4 rounded-full" />
            <SkeletonBox className="h-4 w-20" />
          </div>
          <div className="col-span-2 flex items-center gap-2">
            <SkeletonBox className="h-2 w-2 rounded-full" />
            <SkeletonBox className="h-4 w-24" />
          </div>
          <SkeletonBox className="col-span-1 h-4 w-12" />
          <SkeletonBox className="col-span-2 h-4 w-20" />
          <div className="col-span-1 flex items-center gap-2">
            <SkeletonBox className="h-6 w-6 rounded-full" />
            <SkeletonBox className="h-4 w-16" />
          </div>
        </div>
      ))}
    </div>
  )
}

/**
 * Skeleton for incident cards on kanban board
 * Shows INC-number, title, severity badge, and avatar
 */
export function SkeletonCard({ count = 3 }: { count?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="bg-white border border-border rounded-lg p-4 shadow-sm space-y-3"
        >
          <div className="flex items-center justify-between">
            <SkeletonBox className="h-3 w-16" />
            <SkeletonBox className="h-4 w-12" />
          </div>
          <SkeletonBox className="h-4 w-full" />
          <SkeletonBox className="h-4 w-3/4" />
          <div className="flex items-center justify-between pt-2">
            <SkeletonBox className="h-5 w-20" />
            <SkeletonBox className="h-6 w-6 rounded-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

/**
 * Skeleton for incident detail page
 * Matches breadcrumb, title, tabs, summary, timeline layout
 */
export function SkeletonDetail() {
  return (
    <div className="p-6 space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2">
        <SkeletonBox className="h-4 w-20" />
        <span className="text-text-tertiary">&gt;</span>
        <SkeletonBox className="h-4 w-24" />
      </div>

      {/* Title */}
      <SkeletonBox className="h-8 w-96" />

      {/* Tabs */}
      <div className="flex gap-6 border-b border-border pb-2">
        <SkeletonBox className="h-5 w-20" />
        <SkeletonBox className="h-5 w-24" />
      </div>

      {/* Summary */}
      <div className="space-y-2">
        <SkeletonBox className="h-4 w-full" />
        <SkeletonBox className="h-4 w-3/4" />
      </div>

      {/* Quick Actions */}
      <div className="flex gap-3">
        <SkeletonBox className="h-9 w-32" />
        <SkeletonBox className="h-9 w-28" />
        <SkeletonBox className="h-9 w-36" />
      </div>

      {/* Activity Section */}
      <div className="space-y-4 pt-4">
        <SkeletonBox className="h-6 w-32" />
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex gap-4">
            <SkeletonBox className="h-4 w-12" />
            <SkeletonBox className="h-2 w-2 rounded-full mt-1" />
            <div className="flex-1 space-y-2">
              <SkeletonBox className="h-4 w-full" />
              <SkeletonBox className="h-4 w-2/3" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * Skeleton for properties panel on incident detail page
 * Matches label+value pairs, avatars, and sections
 */
export function SkeletonProperties() {
  return (
    <div className="w-80 border-l border-border bg-white p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <SkeletonBox className="h-5 w-24" />
        <SkeletonBox className="h-5 w-5" />
      </div>

      {/* Properties */}
      <div className="space-y-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="space-y-2">
            <SkeletonBox className="h-3 w-20" />
            <SkeletonBox className="h-5 w-32" />
          </div>
        ))}
      </div>

      {/* Divider */}
      <div className="border-t border-border" />

      {/* Roles Section */}
      <div className="space-y-4">
        <SkeletonBox className="h-4 w-16" />
        {Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <SkeletonBox className="h-8 w-8 rounded-full" />
            <SkeletonBox className="h-4 w-28" />
          </div>
        ))}
      </div>

      {/* Divider */}
      <div className="border-t border-border" />

      {/* Links Section */}
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center justify-between">
            <SkeletonBox className="h-4 w-24" />
            <SkeletonBox className="h-4 w-10" />
          </div>
        ))}
      </div>
    </div>
  )
}
