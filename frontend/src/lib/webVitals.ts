import { onCLS, onINP, onLCP } from 'web-vitals'
import { reportWebVital } from './telemetry'
import type { Metric } from 'web-vitals'

/**
 * Reports Core Web Vitals (LCP, INP, CLS) through the same route-tagged,
 * PII-free path as error reporting (REG-14).
 *
 * Called once at app bootstrap, not per-page: LCP/CLS/INP are measured by
 * the browser against the actual page load, not React Router's client-side
 * route changes, so there is no per-component "only observe this page"
 * scoping to do here — reportWebVital already tags each measurement with
 * window.location.pathname at report time, which is what actually
 * identifies whether a given report came from the incident list, incident
 * detail, or another page.
 */
export function initWebVitalsReporting(): void {
  const forward = (metric: Metric) => {
    reportWebVital({ name: metric.name, value: metric.value, id: metric.id })
  }
  onCLS(forward)
  onINP(forward)
  onLCP(forward)
}
