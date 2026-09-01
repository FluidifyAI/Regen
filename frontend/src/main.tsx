import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import { installUnhandledRejectionReporting } from './lib/telemetry'
import { initWebVitalsReporting } from './lib/webVitals'
import './lib/posthogClient'
import './index.css'

installUnhandledRejectionReporting()
initWebVitalsReporting()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
)
