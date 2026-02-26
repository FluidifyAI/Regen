import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getCurrentUser, type CurrentUser } from '../api/auth'

interface AuthState {
  user: CurrentUser | null
  loading: boolean
  /** true when SAML is configured AND user is authenticated */
  authenticated: boolean
  /** true when SAML is not configured — all requests permitted */
  openMode: boolean
  /** true when SSO/SAML is enabled on the server */
  ssoEnabled: boolean
}

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  authenticated: false,
  openMode: false,
  ssoEnabled: false,
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

  const value: AuthState = {
    user,
    loading,
    authenticated: user?.authenticated === true,
    openMode: user?.mode === 'open',
    ssoEnabled: user?.ssoEnabled === true,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  return useContext(AuthContext)
}
