import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const { captureMock } = vi.hoisted(() => ({ captureMock: vi.fn() }))
vi.mock('./posthogClient', () => ({
  posthog: { capture: captureMock },
}))

import { reportError, reportWebVital, installUnhandledRejectionReporting } from './telemetry'

// A known SHA-256 digest, computed independently, so the hash assertions
// prove the actual algorithm/encoding rather than just "some hex string".
// sha256("boom") = 81f52337ebb4cb1669bb802c708807dde0519d15cb102a6313d26ad5cd821713
const SHA256_OF_BOOM = '81f52337ebb4cb1669bb802c708807dde0519d15cb102a6313d26ad5cd821713'

function firstCapture(): [string, Record<string, unknown>] {
  const call = captureMock.mock.calls[0]
  if (!call) throw new Error('capture was never called')
  return call as [string, Record<string, unknown>]
}

describe('reportError', () => {
  beforeEach(() => {
    captureMock.mockClear()
    window.history.pushState({}, '', '/incidents/42?ref=slack&token=secret-abc')
  })

  it('sends a sha256 hash of the message, matching the known digest for "boom"', async () => {
    await reportError(new Error('boom'))
    expect(captureMock).toHaveBeenCalledTimes(1)
    const [, props] = firstCapture()
    expect(props.message_hash).toBe(SHA256_OF_BOOM)
  })

  it('never includes the raw error message anywhere in the reported payload', async () => {
    const secretMessage = 'incident "Payment gateway outage for customer Acme Corp" failed to load'
    await reportError(new Error(secretMessage))
    const [, props] = firstCapture()
    const serialized = JSON.stringify(props)
    expect(serialized).not.toContain('Payment gateway')
    expect(serialized).not.toContain('Acme Corp')
    expect(serialized).not.toContain(secretMessage)
  })

  it('includes the error name and component stack when provided', async () => {
    await reportError(new TypeError('boom'), '    at IncidentDetail\n    at ErrorBoundary')
    const [, props] = firstCapture()
    expect(props.name).toBe('TypeError')
    expect(props.component_stack).toBe('    at IncidentDetail\n    at ErrorBoundary')
  })

  it('includes the route path but never the query string (which can carry tokens)', async () => {
    await reportError(new Error('boom'))
    const [, props] = firstCapture()
    expect(props.route).toBe('/incidents/42')
    const serialized = JSON.stringify(props)
    expect(serialized).not.toContain('secret-abc')
  })

  it('reports under the $exception event name', async () => {
    await reportError(new Error('boom'))
    const [eventName] = firstCapture()
    expect(eventName).toBe('$exception')
  })
})

describe('reportWebVital', () => {
  beforeEach(() => {
    captureMock.mockClear()
    window.history.pushState({}, '', '/incidents')
  })

  it('reports name, value, id, and route under a web_vital event', () => {
    reportWebVital({ name: 'LCP', value: 1234.5, id: 'v1-abc' })
    expect(captureMock).toHaveBeenCalledWith('web_vital', {
      name: 'LCP',
      value: 1234.5,
      metric_id: 'v1-abc',
      route: '/incidents',
    })
  })
})

describe('installUnhandledRejectionReporting', () => {
  // Rather than dispatching a real "unhandledrejection" event (inconsistent
  // support for constructing PromiseRejectionEvent across jsdom/happy-dom),
  // capture the listener function installUnhandledRejectionReporting
  // registers and invoke it directly with a fake event object — this tests
  // the actual reporting logic without depending on the test environment's
  // fidelity to that specific browser event type.
  //
  // window is shared across test files within a worker, so every installed
  // listener MUST be removed after its test — otherwise it keeps firing for
  // real unhandledrejection events raised anywhere else in the suite.
  let installedListener: EventListenerOrEventListenerObject | undefined

  function captureInstalledListener(): (event: { reason: unknown }) => void {
    const addSpy = vi.spyOn(window, 'addEventListener')
    installUnhandledRejectionReporting()
    const call = addSpy.mock.calls.find(([type]) => type === 'unhandledrejection')
    if (!call) throw new Error('installUnhandledRejectionReporting did not register a listener')
    addSpy.mockRestore()
    installedListener = call[1]
    return call[1] as unknown as (event: { reason: unknown }) => void
  }

  beforeEach(() => {
    captureMock.mockClear()
    window.history.pushState({}, '', '/incidents')
  })

  afterEach(() => {
    if (installedListener) {
      window.removeEventListener('unhandledrejection', installedListener)
      installedListener = undefined
    }
  })

  it('reports an Error reason via reportError', async () => {
    const listener = captureInstalledListener()
    listener({ reason: new Error('boom') })

    // reportError is async (hashing); let its microtask settle.
    await new Promise((r) => setTimeout(r, 0))

    expect(captureMock).toHaveBeenCalledTimes(1)
    const [eventName, props] = firstCapture()
    expect(eventName).toBe('$exception')
    expect(props.message_hash).toBe(SHA256_OF_BOOM)
  })

  it('normalizes a non-Error rejection reason (e.g. a rejected string) without throwing', async () => {
    const listener = captureInstalledListener()
    listener({ reason: 'plain string rejection' })

    await new Promise((r) => setTimeout(r, 0))

    expect(captureMock).toHaveBeenCalledTimes(1)
    const [, props] = firstCapture()
    expect(props.name).toBe('UnhandledRejection')
  })
})
