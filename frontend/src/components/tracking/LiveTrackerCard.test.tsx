import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LiveTrackerCard } from './LiveTrackerCard'
import { BusTrackingInfo } from '../../types/transit'

// Exactly the shape GET /api/v1/bus/:code/track returns (see README).
const apiPayload: BusTrackingInfo = {
  bus_code: 'KS602',
  tracking_url: 'https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602',
  live: {
    bus_code: 'KS602',
    vehicle_code: 'KL15A3159',
    operator: 'KSRTC',
    route_name: 'Kannur-Punalur',
    latitude: 11.205833,
    longitude: 75.811525,
    speed: 42.4,
    recorded_at: new Date(Date.now() - 20_000).toISOString(),
    next_stop: 'Cheruvannur Koyas',
    previous_stop: 'Meenchanda Bypass',
    live_tracking: true,
    tracking_url: 'https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602',
    heading: 92,
    age_seconds: 20,
    stale: false,
  },
}

function renderCard(overrides: Partial<Parameters<typeof LiveTrackerCard>[0]> = {}) {
  const props = {
    data: apiPayload,
    mode: 'live' as const,
    paused: false,
    onTogglePaused: vi.fn(),
    onRefresh: vi.fn(),
    distanceM: 1530,
    trailPoints: 4,
    ...overrides,
  }
  render(<LiveTrackerCard {...props} />)
  return props
}

describe('LiveTrackerCard', () => {
  // Regression: the UI read `position.speed_kmh` / `prev_stop` while the API
  // sends `live.speed` / `previous_stop`, so live data never rendered.
  it('renders the live fields the API actually returns', () => {
    renderCard()
    expect(screen.getByText('42 km/h')).toBeDefined()
    expect(screen.getByText('In Transit')).toBeDefined()
    expect(screen.getAllByText('Cheruvannur Koyas').length).toBeGreaterThan(0)
    expect(screen.getByText('From: Meenchanda Bypass')).toBeDefined()
    expect(screen.getByText('20s ago')).toBeDefined()
    expect(screen.getByText('Live Telemetry')).toBeDefined()
    expect(screen.getByText(/Kannur-Punalur/)).toBeDefined()
    expect(screen.getByText(/KSRTC · KL15A3159/)).toBeDefined()
  })

  it('shows heading as a compass point with distance tracked', () => {
    renderCard()
    expect(screen.getByText('E')).toBeDefined()
    expect(screen.getByText('92° · 1.5 km tracked')).toBeDefined()
  })

  it('flags stale positions', () => {
    renderCard({ data: { ...apiPayload, live: { ...apiPayload.live!, stale: true } } })
    expect(screen.getByText('Stale Signal')).toBeDefined()
    expect(screen.getByText(/^Stale · /)).toBeDefined()
  })

  it('shows the connection mode and toggles pause', () => {
    const props = renderCard({ mode: 'polling' })
    const toggle = screen.getByTestId('connection-mode')
    expect(toggle.textContent).toContain('Polling 15s')
    fireEvent.click(toggle)
    expect(props.onTogglePaused).toHaveBeenCalledOnce()
  })

  it('surfaces a soft warning without hiding the last position', () => {
    renderCard({ warning: { code: 'UPSTREAM_CHALO_ERROR', message: 'Upstream Chalo service failure' } })
    expect(screen.getByRole('status').textContent).toContain('UPSTREAM_CHALO_ERROR')
    expect(screen.getByText('42 km/h')).toBeDefined()
  })

  it('handles a bus without a GPS fix', () => {
    renderCard({ data: { bus_code: 'X1', tracking_url: '' }, trailPoints: 0, distanceM: 0 })
    expect(screen.getByText('No GPS Fix')).toBeDefined()
    expect(screen.getByText('0 km/h')).toBeDefined()
    expect(screen.getByText('Awaiting movement')).toBeDefined()
  })
})

describe('LiveTrackerCard favorites and degraded mode', () => {
  it('toggles the bus in My buses', () => {
    const onToggleFavorite = vi.fn()
    renderCard({ onToggleFavorite })
    const star = screen.getByTestId('favorite-toggle')
    expect(star.getAttribute('aria-pressed')).toBe('false')
    expect(star.getAttribute('aria-label')).toBe('Save KS602 to My buses')
    fireEvent.click(star)
    expect(onToggleFavorite).toHaveBeenCalledOnce()
  })

  it('cannot add to a full list but can still remove', () => {
    renderCard({ onToggleFavorite: vi.fn(), favoriteDisabled: true })
    expect((screen.getByTestId('favorite-toggle') as HTMLButtonElement).disabled).toBe(true)
  })

  it('reports a saved bus as pressed', () => {
    renderCard({ onToggleFavorite: vi.fn(), isFavorite: true, favoriteDisabled: true })
    const star = screen.getByTestId('favorite-toggle') as HTMLButtonElement
    expect(star.getAttribute('aria-pressed')).toBe('true')
    expect(star.disabled).toBe(false)
  })

  it('makes a degraded (last known) position unmistakable', () => {
    renderCard({
      data: { ...apiPayload, degraded: true },
      warning: { code: 'UPSTREAM_CHALO_ERROR', message: 'Upstream Chalo service failure' },
    })
    expect(screen.getByText('Last Known Position')).toBeDefined()
    expect(screen.getByTestId('degraded-banner').textContent).toContain("Chalo isn't responding")
    // One explanation at a time: the generic warning is folded into the banner.
    expect(screen.queryByText(/Live feed issue/)).toBeNull()
  })
})
