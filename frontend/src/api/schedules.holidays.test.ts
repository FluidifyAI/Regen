import { describe, it, expect, vi, beforeEach } from 'vitest'

const { get, post, patch, del } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() }))
vi.mock('./client', () => ({ apiClient: { get, post, patch, delete: del } }))

import { getHolidays, createHoliday, updateHoliday, deleteHoliday } from './schedules'

describe('holiday API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('only asks for hidden holidays when told to', async () => {
    await getHolidays('s1', '2026-10-01', '2026-11-01')
    expect(get).toHaveBeenLastCalledWith('/api/v1/schedules/s1/holidays?from=2026-10-01&to=2026-11-01')

    await getHolidays('s1', '2026-10-01', '2026-11-01', true)
    expect(get).toHaveBeenLastCalledWith('/api/v1/schedules/s1/holidays?from=2026-10-01&to=2026-11-01&include_hidden=true')
  })

  it('creates, updates and deletes through the schedule-scoped endpoints', async () => {
    await createHoliday('s1', { date: '2026-11-03', name: 'Company day' })
    expect(post).toHaveBeenCalledWith('/api/v1/schedules/s1/holidays', { date: '2026-11-03', name: 'Company day' })

    await updateHoliday('s1', 'h1', { hidden: true })
    expect(patch).toHaveBeenCalledWith('/api/v1/schedules/s1/holidays/h1', { hidden: true })

    await deleteHoliday('s1', 'h1')
    expect(del).toHaveBeenCalledWith('/api/v1/schedules/s1/holidays/h1')
  })
})
