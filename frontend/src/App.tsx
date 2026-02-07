import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppLayout } from './components/layout/AppLayout'
import { HomePage } from './pages/HomePage'
import { IncidentsListPage } from './pages/IncidentsListPage'
import { IncidentDetailPage } from './pages/IncidentDetailPage'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/incidents" element={<IncidentsListPage />} />
          <Route path="/incidents/:id" element={<IncidentDetailPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
