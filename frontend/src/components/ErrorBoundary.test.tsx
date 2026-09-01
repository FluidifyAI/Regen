import { describe, it, expect, vi, beforeEach } from 'vitest'

const { reportErrorMock } = vi.hoisted(() => ({ reportErrorMock: vi.fn() }))
vi.mock('../lib/telemetry', () => ({
  reportError: reportErrorMock,
}))

import { ErrorBoundary } from './ErrorBoundary'

describe('ErrorBoundary.componentDidCatch', () => {
  beforeEach(() => {
    reportErrorMock.mockClear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('reports the caught error and component stack via reportError', () => {
    const boundary = new ErrorBoundary({ children: null })
    const error = new Error('render blew up')
    const errorInfo: React.ErrorInfo = { componentStack: '    at IncidentDetail\n    at ErrorBoundary' }

    boundary.componentDidCatch(error, errorInfo)

    expect(reportErrorMock).toHaveBeenCalledTimes(1)
    expect(reportErrorMock).toHaveBeenCalledWith(error, '    at IncidentDetail\n    at ErrorBoundary')
  })

  it('still logs to console (existing behavior preserved)', () => {
    const boundary = new ErrorBoundary({ children: null })
    boundary.componentDidCatch(new Error('x'), { componentStack: '' })
    expect(console.error).toHaveBeenCalled()
  })
})
