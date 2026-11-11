import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RouteDetailsTab } from './RouteDetailsTab'

describe('RouteDetailsTab', () => {
  it('renders clean fallback empty state when routeData is missing', () => {
    render(<RouteDetailsTab busCode="KS602" routeData={null} />)

    expect(screen.getByText('Route Endpoints Not Captured')).toBeDefined()
    expect(screen.getByText(/Telemetry Only Mode/)).toBeDefined()
    expect(screen.getByText(/KS602/)).toBeDefined()
  })

  it('renders loading state when isLoading is true', () => {
    const { container } = render(<RouteDetailsTab busCode="KS602" isLoading={true} />)
    expect(container.querySelector('.animate-pulse')).toBeDefined()
  })

  it('renders route and tracking endpoint information when routeData is present', () => {
    const mockData = {
      bus_code: 'KS602',
      tracking_url: 'https://chalo.com/track/ks602',
      route_url: 'https://chalo.com/routedetailslive/ks602',
    }

    render(<RouteDetailsTab busCode="KS602" routeData={mockData} />)

    expect(screen.getByText(/Discovered Route Endpoints for KS602/)).toBeDefined()
    expect(screen.getByDisplayValue('https://chalo.com/routedetailslive/ks602')).toBeDefined()
    expect(screen.getByDisplayValue('https://chalo.com/track/ks602')).toBeDefined()
  })
})
