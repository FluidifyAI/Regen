import { describe, it, expect, vi } from 'vitest'

const { onCLSMock, onINPMock, onLCPMock, reportWebVitalMock } = vi.hoisted(() => ({
  onCLSMock: vi.fn(),
  onINPMock: vi.fn(),
  onLCPMock: vi.fn(),
  reportWebVitalMock: vi.fn(),
}))
vi.mock('web-vitals', () => ({
  onCLS: onCLSMock,
  onINP: onINPMock,
  onLCP: onLCPMock,
}))
vi.mock('./telemetry', () => ({
  reportWebVital: reportWebVitalMock,
}))

import { initWebVitalsReporting } from './webVitals'

describe('initWebVitalsReporting', () => {
  it('registers a callback with each of onCLS, onINP, and onLCP', () => {
    initWebVitalsReporting()
    expect(onCLSMock).toHaveBeenCalledTimes(1)
    expect(onINPMock).toHaveBeenCalledTimes(1)
    expect(onLCPMock).toHaveBeenCalledTimes(1)
  })

  it('forwards a reported metric to reportWebVital with name/value/id', () => {
    initWebVitalsReporting()
    const call = onLCPMock.mock.calls[0]
    if (!call) throw new Error('onLCP was never called')
    const lcpCallback = call[0]
    lcpCallback({ name: 'LCP', value: 2100.4, id: 'v3-abc', rating: 'good' })

    expect(reportWebVitalMock).toHaveBeenCalledWith({ name: 'LCP', value: 2100.4, id: 'v3-abc' })
  })
})
