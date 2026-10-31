import { createContext } from 'react'
import type { CurrentUser } from '../api/auth'

export interface AuthState {
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

export const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  authenticated: false,
  openMode: false,
  ssoEnabled: false,
  signOut: async () => {},
  refresh: async () => {},
})
