import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { GroupedAlerts } from './GroupedAlerts'
import type { Alert, Incident } from '../../api/types'

const incident = { id: 'incident' } as Incident
const alert = {
  id: 'alert',
  title: 'Bridge alert',
  source: 'generic',
  severity: 'warning',
  status: 'firing',
  labels: null,
} as Alert

describe('linked alert date rendering', () => {
  it.each([undefined, null, '', 'invalid'])(
    'falls back from %s without displaying invalid dates',
    (started_at) => {
      const html = renderToStaticMarkup(
        <GroupedAlerts
          incident={incident}
          alerts={[
            { ...alert, started_at, received_at: '2026-10-28T05:26:41Z', ended_at: 'invalid' },
          ]}
        />
      )
      expect(html).toContain('Oct 28')
      expect(html).not.toMatch(/Invalid Date|NaN|1970|Resolved /)
    }
  )
  it('shows a placeholder when neither start nor receipt is usable', () => {
    const html = renderToStaticMarkup(
      <GroupedAlerts
        incident={incident}
        alerts={[{ ...alert, started_at: null, received_at: '' }]}
      />
    )
    expect(html).toContain('—')
    expect(html).not.toMatch(/Invalid Date|NaN|1970/)
  })
  it('displays a valid resolution timestamp', () => {
    const html = renderToStaticMarkup(
      <GroupedAlerts
        incident={incident}
        alerts={[
          {
            ...alert,
            status: 'resolved',
            started_at: '2026-10-28T05:26:41Z',
            ended_at: '2026-10-28T07:30:00.123+01:00',
          },
        ]}
      />
    )
    expect(html).toContain('Resolved Oct 28')
  })
})
