import { type ReactNode, useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../../hooks/useAuth'
import { LoginPage } from '../../pages/LoginPage'
import { getSetupStatus } from '../../api/setup'
import { LOGO_ICON_DATA_URI } from '../../assets/logoIcon'

// Routes that are always accessible regardless of auth state.
const PUBLIC_PATHS = ['/login', '/logout', '/status']

const WIZARD_STORAGE_KEY = 'regen_setup_wizard_v1'

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
 * On first authenticated render, checks whether the onboarding wizard
 * has been dismissed. If not and Slack isn't connected, redirects to /setup.
 *
 * Open mode (no users exist yet) no longer bypasses auth. The LoginPage
 * detects open mode and shows the first-run setup form automatically.
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { loading, authenticated } = useAuth()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const setupChecked = useRef(false)

  useEffect(() => {
    if (!authenticated || setupChecked.current || pathname === '/setup') return
    if (localStorage.getItem(WIZARD_STORAGE_KEY)) {
      setupChecked.current = true
      return
    }
    getSetupStatus()
      .then(status => {
        setupChecked.current = true
        if (!status.slack_connected) navigate('/setup')
      })
      .catch(() => { setupChecked.current = true })
  }, [authenticated, pathname, navigate])

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
          <img src={LOGO_ICON_DATA_URI} alt="Regen" className="relative w-10 h-10" />
        </div>
        <div className="text-sm font-bold tracking-tight text-[#1E293B]">
          Regen
        </div>
      </div>
    </div>
  )
}
