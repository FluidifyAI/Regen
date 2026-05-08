import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import { posthog } from '../../main'
import { Sidebar } from './Sidebar'
import { AnnouncementBanner } from '../AnnouncementBanner'
import { getSystemSettings } from '../../api/settings'

/**
 * App shell layout with persistent sidebar and main content area.
 * On mount, fetches system settings to opt PostHog in/out based on admin preference.
 */
export function AppLayout() {
  useEffect(() => {
    getSystemSettings()
      .then((s) => {
        if (s.telemetry_enabled) {
          posthog.opt_in_capturing()
        } else {
          posthog.opt_out_capturing()
        }
      })
      .catch(() => {})
  }, [])

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />

      <div className="flex flex-col flex-1 md:ml-60 transition-all duration-200 overflow-hidden">
        <AnnouncementBanner />

        {/* Desktop-only notice — visible on small viewports */}
        <div className="md:hidden flex items-center justify-center gap-2 px-4 py-2.5 bg-amber-50 border-b border-amber-200 text-xs text-amber-800 font-medium">
          <span>🖥️</span>
          <span>Best experienced on a desktop browser</span>
        </div>

        {/* Main Content Area */}
        <main className="flex-1 overflow-y-auto bg-white">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
