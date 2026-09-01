import { describe, it, expect } from 'vitest'
import { generateTraceparent, shouldInjectTraceparent } from './tracing'

describe('generateTraceparent', () => {
  it('matches the W3C traceparent format: version-traceId-spanId-flags', () => {
    const header = generateTraceparent()
    expect(header).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/)
  })

  it('produces a different trace-id and span-id on each call', () => {
    const a = generateTraceparent()
    const b = generateTraceparent()
    expect(a).not.toBe(b)
  })

  it('never produces the all-zero trace-id or span-id (both explicitly invalid per the W3C spec)', () => {
    // Run enough times that a real bug (e.g. an off-by-one truncating to zero)
    // would show up, without asserting anything about true randomness quality.
    for (let i = 0; i < 50; i++) {
      const header = generateTraceparent()
      const [, traceId, spanId] = header.split('-')
      expect(traceId).not.toBe('0'.repeat(32))
      expect(spanId).not.toBe('0'.repeat(16))
    }
  })
})

describe('shouldInjectTraceparent', () => {
  const origin = window.location.origin

  it('returns true for a same-origin relative /api/v1/* path', () => {
    expect(shouldInjectTraceparent('/api/v1/incidents')).toBe(true)
  })

  it('returns true for a same-origin absolute /api/v1/* URL', () => {
    expect(shouldInjectTraceparent(`${origin}/api/v1/incidents/123`)).toBe(true)
  })

  it('returns false for a same-origin path outside /api/v1', () => {
    expect(shouldInjectTraceparent('/health')).toBe(false)
    expect(shouldInjectTraceparent('/metrics')).toBe(false)
  })

  it('returns false for a cross-origin URL, even under /api/v1', () => {
    expect(shouldInjectTraceparent('https://evil.example.com/api/v1/incidents')).toBe(false)
  })

  it('returns false for an unparseable URL', () => {
    expect(shouldInjectTraceparent('::not a url::')).toBe(false)
  })
})
