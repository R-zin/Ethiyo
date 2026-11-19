import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RouteDetailsTab } from './RouteDetailsTab'

describe('RouteDetailsTab', () => {
  it('explains the wait while the route is being discovered', () => {
    render(<RouteDetailsTab busCode="KS602" isLoading />)
    expect(screen.getByTestId('route-loading').textContent).toContain('headless browser')
  })

  // Regression: a failed discovery used to render an empty panel.
  it('shows discovery failures instead of a blank panel', () => {
    render(<RouteDetailsTab busCode="KS602" error={{ code: 'CHALO_TIMEOUT', message: 'Timed out communicating with Chalo' }} />)
    const panel = screen.getByTestId('route-error')
    expect(panel.textContent).toContain('CHALO_TIMEOUT')
    expect(panel.textContent).toContain('Live tracking is unaffected')
  })

  it('lists the discovered endpoints', () => {
    render(
      <RouteDetailsTab
        busCode="KS602"
        routeData={{ bus_code: 'KS602', tracking_url: 'https://t.example/raw', route_url: 'https://r.example/routedetailslive' }}
      />
    )
    expect((screen.getByLabelText('Scheduler Live Route URL') as HTMLInputElement).value).toBe('https://r.example/routedetailslive')
    expect((screen.getByLabelText('Live Tracking Telemetry URL') as HTMLInputElement).value).toBe('https://t.example/raw')
  })
})
