import React from 'react'
import ReactDOM from 'react-dom/client'
import posthog from 'posthog-js'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import './index.css'

// Start opted-out; AppLayout opts-in after confirming telemetry_enabled from settings API.
// This ensures no data is captured before the admin's preference is known.
posthog.init('phc_PLACEHOLDER', {
  api_host: 'https://us.i.posthog.com',
  capture_pageview: true,
  autocapture: false,
  persistence: 'memory',
  opt_out_capturing_by_default: true,
})

export { posthog }

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
)
