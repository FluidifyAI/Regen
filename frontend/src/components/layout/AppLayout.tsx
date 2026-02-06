import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'

/**
 * App shell layout with persistent sidebar and main content area
 * Sidebar persists across all routes, content area renders nested routes
 */
export function AppLayout() {
  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />

      {/* Main Content Area */}
      <main className="flex-1 overflow-y-auto bg-white md:ml-60 transition-all duration-200">
        <Outlet />
      </main>
    </div>
  )
}
