import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
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

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AuthGate>
          <Routes>
            {/* Standalone login route — rendered by AuthGate when unauthed, but also
                directly accessible so the browser can land here after /auth/logout */}
            <Route path="/login" element={<LoginPage />} />

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
              <Route path="/settings/users" element={<SettingsUsersPage />} />
            </Route>
          </Routes>
        </AuthGate>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
