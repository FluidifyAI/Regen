import { useContext } from 'react'
import { AuthContext, type AuthState } from '../contexts/authState'

export function useAuth(): AuthState {
  return useContext(AuthContext)
}
