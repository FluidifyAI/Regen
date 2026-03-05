import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from 'react'
import { getCurrentUser, logout as apiLogout, type CurrentUser } from '../api/auth'

interface AuthState {
  user: CurrentUser | null
  loading: boolean
  /** true when SAML is configured AND user is authenticated */
  authenticated: boolean
  /** true when SAML is not configured — all requests permitted */
  openMode: boolean
  /** true when SSO/SAML is enabled on the server */
  ssoEnabled: boolean
  /** Call to log out: clears server session then resets auth state. */
  signOut: () => Promise<void>
  /** Re-fetch the current user (e.g. after a profile update). */
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  authenticated: false,
  openMode: false,
  ssoEnabled: false,
  signOut: async () => {},
  refresh: async () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getCurrentUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  const refresh = useCallback(async () => {
    try {
      const u = await getCurrentUser()
      setUser(u)
    } catch {
      // ignore
    }
  }, [])

  const signOut = useCallback(async () => {
    try {
      await apiLogout()
    } catch {
      // session may already be gone — proceed regardless
    }
    setUser(null)
  }, [])

  const value: AuthState = {
    user,
    loading,
    authenticated: user?.authenticated === true,
    openMode: user?.mode === 'open',
    ssoEnabled: user?.ssoEnabled === true,
    signOut,
    refresh,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  return useContext(AuthContext)
}
