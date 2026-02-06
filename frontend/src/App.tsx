import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppLayout } from './components/layout/AppLayout'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/incidents" element={<IncidentsPage />} />
          <Route path="/incidents/:id" element={<IncidentDetailPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

// Placeholder components
function HomePage() {
  return (
    <div className="flex h-screen items-center justify-center bg-slate-50">
      <div className="text-center">
        <h1 className="text-3xl font-bold text-slate-900">OpenIncident</h1>
        <p className="mt-2 text-slate-600">Home Dashboard (Coming Soon)</p>
      </div>
    </div>
  )
}

function IncidentsPage() {
  return (
    <div className="flex h-screen items-center justify-center bg-slate-50">
      <div className="text-center">
        <h1 className="text-3xl font-bold text-slate-900">Incidents</h1>
        <p className="mt-2 text-slate-600">Incidents List (Coming Soon)</p>
      </div>
    </div>
  )
}

function IncidentDetailPage() {
  return (
    <div className="flex h-screen items-center justify-center bg-slate-50">
      <div className="text-center">
        <h1 className="text-3xl font-bold text-slate-900">Incident Detail</h1>
        <p className="mt-2 text-slate-600">Incident Detail Page (Coming Soon)</p>
      </div>
    </div>
  )
}

export default App
