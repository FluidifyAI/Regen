import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiClient } from './client'

function firstCallHeaders(mockFetch: ReturnType<typeof vi.fn>): Record<string, string> {
  const call = mockFetch.mock.calls[0]
  if (!call) throw new Error('fetch was never called')
  const init = call[1] as RequestInit
  return init.headers as Record<string, string>
}

describe('ApiClient traceparent injection', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    globalThis.fetch = vi.fn(
      async () =>
        new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('injects a traceparent header on /api/v1/* requests', async () => {
    await apiClient.get('/api/v1/incidents')

    const mockFetch = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
    expect(mockFetch).toHaveBeenCalledTimes(1)
    const headers = firstCallHeaders(mockFetch)
    expect(headers.traceparent).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/)
  })

  it('does not inject a traceparent header on non-API requests', async () => {
    // apiClient always targets BASE_URL + endpoint, so a request to a path
    // outside /api/v1 is the realistic negative case (e.g. a future health
    // check helper reusing the same request() method).
    await apiClient.get('/health')

    const mockFetch = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
    const headers = firstCallHeaders(mockFetch)
    expect(headers.traceparent).toBeUndefined()
  })

  it('still sets Content-Type and preserves caller-supplied headers alongside traceparent', async () => {
    await apiClient.post('/api/v1/incidents', { title: 'x' })

    const mockFetch = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
    const headers = firstCallHeaders(mockFetch)
    expect(headers['Content-Type']).toBe('application/json')
    expect(headers.traceparent).toBeDefined()
  })
})
