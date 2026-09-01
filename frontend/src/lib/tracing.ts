/**
 * Lightweight W3C trace-context propagation for the browser (REG-14).
 *
 * A full OpenTelemetry Web SDK (@opentelemetry/sdk-trace-web +
 * exporter-trace-otlp-http + instrumentation-fetch + ...) would let browser
 * actions become their own exportable spans, but that isn't what this
 * ticket needs: the goal is trace *continuity* — one browser action
 * produces a single trace that starts in the browser and continues into the
 * backend — not browser-side span export to a collector. The backend
 * (otelgin, via observability.GinMiddleware) already extracts an incoming
 * `traceparent` header and parents its span tree under it; all the browser
 * needs to do is generate and send one.
 */

/** Returns a random hex string of `bytes` bytes (2 hex chars per byte). */
function randomHex(bytes: number): string {
  const arr = new Uint8Array(bytes)
  crypto.getRandomValues(arr)
  return Array.from(arr, (b) => b.toString(16).padStart(2, '0')).join('')
}

/**
 * Generates a fresh W3C `traceparent` header value: a new 16-byte trace-id
 * and 8-byte parent-id, version `00`, sampled flag set (`01`) — see
 * https://www.w3.org/TR/trace-context/#traceparent-header.
 *
 * A new trace-id is generated per request (not per page load): each API
 * call is its own root trace from the backend's perspective, matching how
 * otelgin treats every inbound request today.
 */
export function generateTraceparent(): string {
  return `00-${randomHex(16)}-${randomHex(8)}-01`
}

/**
 * Reports whether `url` is a same-origin `/api/v1/*` request — the only
 * requests that should carry a traceparent header. Trace IDs are not
 * sensitive, but sending them to third-party origins (analytics beacons,
 * CDNs, OAuth providers) is pointless at best and a fingerprinting-adjacent
 * habit at worst, so this stays scoped to Regen's own API.
 */
export function shouldInjectTraceparent(url: string): boolean {
  try {
    const parsed = new URL(url, window.location.origin)
    return parsed.origin === window.location.origin && parsed.pathname.startsWith('/api/v1/')
  } catch {
    return false
  }
}
