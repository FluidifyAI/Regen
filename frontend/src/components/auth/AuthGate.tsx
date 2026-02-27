import type { ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../hooks/useAuth'
import { LoginPage } from '../../pages/LoginPage'

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
 * │ Open mode (no auth)          │ Render children (passthrough)│
 * │ Authenticated                │ Render children              │
 * │ Unauthenticated              │ Show LoginPage               │
 * └──────────────────────────────┴──────────────────────────────┘
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { loading, authenticated, openMode } = useAuth()
  const { pathname } = useLocation()

  // Always let public pages render, even before the auth check completes.
  if (PUBLIC_PATHS.includes(pathname)) {
    return <>{children}</>
  }

  if (loading) {
    return <AuthLoadingScreen />
  }

  // Open mode or authenticated — let the app render
  if (openMode || authenticated) {
    return <>{children}</>
  }

  // Auth required (local users exist or SAML configured), no active session → show login
  return <LoginPage />
}

function AuthLoadingScreen() {
  return (
    <div className="fixed inset-0 bg-[#0d0f14] flex items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        {/* Pulsing shield logo */}
        <div className="relative">
          <div className="absolute inset-0 rounded-full bg-blue-500 opacity-20 animate-ping" />
          <svg
            className="w-10 h-10 text-blue-500 relative"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
            />
          </svg>
        </div>
        <div className="text-[#4a5568] text-sm tracking-widest uppercase">
          OpenIncident
        </div>
      </div>
    </div>
  )
}
