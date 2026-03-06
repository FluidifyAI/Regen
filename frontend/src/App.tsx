import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useParams } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { GlobalSearch } from './components/GlobalSearch'
import { AuthGate } from './components/auth/AuthGate'
import { AppLayout } from './components/layout/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { HomePage } from './pages/HomePage'
import { IncidentsListPage } from './pages/IncidentsListPage'
import { IncidentDetailPage } from './pages/IncidentDetailPage'
import { OnCallPage } from './pages/OnCallPage'
import { ScheduleDetailPage } from './pages/ScheduleDetailPage'
import { EscalationPolicyDetailPage } from './pages/EscalationPolicyDetailPage'
import { AlertDetailPage } from './pages/AlertDetailPage'
import { PostMortemTemplatesPage } from './pages/PostMortemTemplatesPage'
import { SettingsUsersPage } from './pages/SettingsUsersPage'
import { SystemSettingsPage } from './pages/SystemSettingsPage'
import { AnalyticsPage } from './pages/AnalyticsPage'
import { IntegrationsPage } from './pages/IntegrationsPage'
import { LogoutPage } from './pages/LogoutPage'

/** Redirect /escalation-policies/:id → /on-call/escalation-paths/:id */
function EscalationPoliciesRedirect() {
  const { id } = useParams<{ id: string }>()
  return <Navigate to={`/on-call/escalation-paths/${id ?? ''}`} replace />
}

function App() {
  const [showSearch, setShowSearch] = useState(false)

  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setShowSearch(true)
      }
    }
    const handleOpenSearch = () => setShowSearch(true)
    document.addEventListener('keydown', handleKeydown)
    window.addEventListener('open-search', handleOpenSearch)
    return () => {
      document.removeEventListener('keydown', handleKeydown)
      window.removeEventListener('open-search', handleOpenSearch)
    }
  }, [])

  return (
    <BrowserRouter>
      <AuthProvider>
        <GlobalSearch isOpen={showSearch} onClose={() => setShowSearch(false)} />
        <AuthGate>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            {/* Shown after successful sign-out — always accessible, bypasses AuthGate */}
            <Route path="/logout" element={<LogoutPage />} />

            <Route element={<AppLayout />}>
              <Route path="/" element={<HomePage />} />
              <Route path="/incidents" element={<IncidentsListPage />} />
              <Route path="/incidents/:id" element={<IncidentDetailPage />} />

              {/* On-call unified section */}
              <Route path="/on-call" element={<OnCallPage />} />
              <Route path="/on-call/escalation-paths" element={<OnCallPage />} />
              <Route path="/on-call/schedules" element={<OnCallPage />} />
              {/* Detail pages (nested under on-call) */}
              <Route path="/on-call/escalation-paths/:id" element={<EscalationPolicyDetailPage />} />
              <Route path="/on-call/:id" element={<ScheduleDetailPage />} />

              {/* Backwards-compat redirects */}
              <Route path="/routing-rules" element={<Navigate to="/on-call" replace />} />
              <Route path="/escalation-policies" element={<Navigate to="/on-call/escalation-paths" replace />} />
              <Route path="/escalation-policies/:id" element={<EscalationPoliciesRedirect />} />

              <Route path="/alerts/:id" element={<AlertDetailPage />} />
              <Route path="/post-mortem-templates" element={<PostMortemTemplatesPage />} />
              <Route path="/analytics" element={<AnalyticsPage />} />
              <Route path="/integrations" element={<IntegrationsPage />} />
              <Route path="/settings" element={<Navigate to="/settings/users" replace />} />
              <Route path="/settings/users" element={<SettingsUsersPage />} />
              <Route path="/settings/system" element={<SystemSettingsPage />} />
            </Route>
          </Routes>
        </AuthGate>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
