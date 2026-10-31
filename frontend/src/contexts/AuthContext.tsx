import { AuthContext, type AuthState } from './authState'
import { useEffect, useState, useCallback, type ReactNode } from 'react'
import { getCurrentUser, logout as apiLogout, type CurrentUser } from '../api/auth'

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
