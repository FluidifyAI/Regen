import posthog from 'posthog-js'

// Start opted-out; AppLayout opts-in after confirming telemetry_enabled from
// settings API. This ensures no data is captured before the admin's
// preference is known — self-hosted-friendly by construction (REG-14): every
// capture() call anywhere in the app (including src/lib/telemetry.ts)
// inherits this same gate for free, with no separate on/off flag to keep in
// sync.
//
// Lives in its own module (not main.tsx, where this used to be defined) so
// telemetry.ts can import the singleton without a circular dependency:
// main.tsx renders ErrorBoundary, which needs to call into telemetry.ts,
// which needs this client — main.tsx -> ErrorBoundary -> telemetry ->
// main.tsx would otherwise be a cycle.
posthog.init('phc_tVN68RCF5waqZs2vqwmCTnuPf8htLDuSUfrezsRpnah2', {
  api_host: 'https://us.i.posthog.com',
  capture_pageview: true,
  autocapture: false,
  persistence: 'memory',
  opt_out_capturing_by_default: true,
})

export { posthog }
