import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppLayout } from './components/layout/AppLayout'
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

function App() {
  return (
    <BrowserRouter>
      <Routes>
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
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
