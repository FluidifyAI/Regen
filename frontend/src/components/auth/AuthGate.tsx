import type { ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../hooks/useAuth'
import { LoginPage } from '../../pages/LoginPage'
import { Bell } from 'lucide-react'

// Routes that are always accessible regardless of auth state.
const PUBLIC_PATHS = ['/login', '/logout']

/**
 * AuthGate wraps the entire app.
 *
 * Behaviour matrix:
 * ┌──────────────────────────────┬──────────────────────────────┐
 * │ State                        │ Result                       │
 * ├──────────────────────────────┼──────────────────────────────┤
 * │ Public path (/login, /logout)│ Render children (passthrough)│
 * │ Loading                      │ Full-screen skeleton         │
 * │ Authenticated                │ Render children              │
 * │ Unauthenticated (open mode)  │ Show LoginPage (setup form)  │
 * │ Unauthenticated              │ Show LoginPage               │
 * └──────────────────────────────┴──────────────────────────────┘
 *
 * Open mode (no users exist yet) no longer bypasses auth. The LoginPage
 * detects open mode and shows the first-run setup form automatically.
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { loading, authenticated } = useAuth()
  const { pathname } = useLocation()

  // Always let public pages render, even before the auth check completes.
  if (PUBLIC_PATHS.includes(pathname)) {
    return <>{children}</>
  }

  if (loading) {
    return <AuthLoadingScreen />
  }

  if (authenticated) {
    return <>{children}</>
  }

  // Not authenticated — show login. LoginPage reads openMode to decide
  // whether to show the sign-in form or the first-run account setup form.
  return <LoginPage />
}

function AuthLoadingScreen() {
  return (
    <div className="fixed inset-0 flex items-center justify-center" style={{ backgroundColor: '#F1F5F9' }}>
      <div className="flex flex-col items-center gap-4">
        <div className="relative flex items-center justify-center">
          <div className="absolute inset-0 rounded-full opacity-20 animate-ping scale-125" style={{ backgroundColor: '#f55609' }} />
          <Bell className="relative w-10 h-10" style={{ color: '#f55609' }} />
        </div>
        <div className="text-sm font-bold tracking-tight text-[#1E293B]">
          Fluidify Regen
        </div>
      </div>
    </div>
  )
}
