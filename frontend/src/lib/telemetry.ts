import { posthog } from './posthogClient'

/**
 * Error and RUM reporting for the frontend (REG-14), with PII stripped
 * before anything reaches PostHog:
 *
 * - The error's raw message is never sent — only a sha256 hash of it, so
 *   identical errors still group/dedupe in PostHog without the message text
 *   (or anything a caller interpolated into it — an incident title, an
 *   email address) ever leaving the browser. Same "hash instead of raw
 *   text" pattern used for LLM prompts in REG-12.
 * - The route path is sent, but never the query string, which can carry
 *   tokens or search terms.
 * - The React component stack is sent as-is: it is always just component
 *   names and file/line locations, never props, state, or rendered data.
 *
 * Every capture() call here inherits posthogClient's existing opt-in gate
 * (opted out until AppLayout confirms telemetry_enabled from the backend) —
 * self-hosted-friendly by construction, nothing new to keep in sync.
 */

async function sha256Hex(text: string): Promise<string> {
  const bytes = new TextEncoder().encode(text)
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('')
}

/** Reports an error (an unhandled React render error, or a promise rejection) with PII stripped. */
export async function reportError(error: Error, componentStack?: string): Promise<void> {
  const messageHash = await sha256Hex(error.message)
  posthog.capture('$exception', {
    name: error.name,
    message_hash: messageHash,
    component_stack: componentStack,
    route: window.location.pathname,
  })
}

/** Normalizes an arbitrary promise-rejection reason into an Error, since `reason` is not guaranteed to be one. */
function toError(reason: unknown): Error {
  if (reason instanceof Error) {
    return reason
  }
  const err = new Error(typeof reason === 'string' ? reason : 'Non-Error promise rejection')
  err.name = 'UnhandledRejection'
  return err
}

/**
 * Installs a global `unhandledrejection` listener that reports through the
 * same PII-stripped path as reportError. ErrorBoundary cannot catch these —
 * React error boundaries only catch errors thrown during rendering,
 * lifecycle methods, and constructors, never inside a promise chain — so
 * this is a separate, necessary listener, not a gap in ErrorBoundary.
 * Call once, at app bootstrap (main.tsx).
 */
export function installUnhandledRejectionReporting(): void {
  window.addEventListener('unhandledrejection', (event) => {
    void reportError(toError(event.reason))
  })
}

export interface WebVitalMetric {
  name: string
  value: number
  id: string
}

/** Reports one Core Web Vitals measurement (LCP, INP, or CLS), tagged with the current route. */
export function reportWebVital(metric: WebVitalMetric): void {
  posthog.capture('web_vital', {
    name: metric.name,
    value: metric.value,
    metric_id: metric.id,
    route: window.location.pathname,
  })
}
