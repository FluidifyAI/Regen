import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { GlobalSearch } from './components/GlobalSearch'
import { AuthGate } from './components/auth/AuthGate'
import { AppLayout } from './components/layout/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { HomePage } from './pages/HomePage'
import { IncidentsListPage } from './pages/IncidentsListPage'
import { IncidentDetailPage } from './pages/IncidentDetailPage'
import { RoutingRulesPage } from './pages/RoutingRulesPage'
import { SchedulesPage } from './pages/SchedulesPage'
import { ScheduleDetailPage } from './pages/ScheduleDetailPage'
import { EscalationPoliciesPage } from './pages/EscalationPoliciesPage'
import { EscalationPolicyDetailPage } from './pages/EscalationPolicyDetailPage'
import { AlertDetailPage } from './pages/AlertDetailPage'
import { PostMortemTemplatesPage } from './pages/PostMortemTemplatesPage'
import { SettingsUsersPage } from './pages/SettingsUsersPage'
import { AnalyticsPage } from './pages/AnalyticsPage'
import { LogoutPage } from './pages/LogoutPage'

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
              <Route path="/routing-rules" element={<RoutingRulesPage />} />
              <Route path="/on-call" element={<SchedulesPage />} />
              <Route path="/on-call/:id" element={<ScheduleDetailPage />} />
              <Route path="/escalation-policies" element={<EscalationPoliciesPage />} />
              <Route path="/escalation-policies/:id" element={<EscalationPolicyDetailPage />} />
              <Route path="/alerts/:id" element={<AlertDetailPage />} />
              <Route path="/post-mortem-templates" element={<PostMortemTemplatesPage />} />
              <Route path="/analytics" element={<AnalyticsPage />} />
              <Route path="/settings" element={<Navigate to="/settings/users" replace />} />
              <Route path="/settings/users" element={<SettingsUsersPage />} />
            </Route>
          </Routes>
        </AuthGate>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
