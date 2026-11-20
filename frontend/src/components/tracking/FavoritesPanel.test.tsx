import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { FavoritesPanel, fleetStatus } from './FavoritesPanel'
import { BusTrackingInfo } from '../../types/transit'

const tracking = (over: Partial<BusTrackingInfo> = {}, stale = false): BusTrackingInfo => ({
  bus_code: 'KS602',
  tracking_url: 't',
  live: {
    bus_code: 'KS602',
    latitude: 10,
    longitude: 76,
    speed: 31,
    route_name: 'Kannur-Punalur',
    next_stop: 'Thalassery',
    recorded_at: new Date(Date.now() - 12_000).toISOString(),
    live_tracking: true,
    tracking_url: 't',
    age_seconds: 12,
    stale,
  },
  ...over,
})

describe('fleetStatus', () => {
  it('labels every state honestly', () => {
    expect(fleetStatus(undefined)).toEqual({ label: 'Loading…', variant: 'neutral' })
    expect(fleetStatus({ tracking: tracking() })).toEqual({ label: 'Live', variant: 'active' })
    expect(fleetStatus({ tracking: tracking({}, true) }).label).toBe('Stale')
    expect(fleetStatus({ tracking: tracking({ degraded: true }) }).label).toBe('Last known')
    expect(fleetStatus({ tracking: { bus_code: 'X', tracking_url: '' } }).label).toBe('No GPS fix')
    expect(fleetStatus({ error: { code: 'BUS_NOT_FOUND', message: 'x' } }).label).toBe('Not live')
    expect(fleetStatus({ error: { code: 'UPSTREAM_UNAVAILABLE', message: 'x' } })).toEqual({ label: 'Unavailable', variant: 'error' })
  })
})

describe('FavoritesPanel', () => {
  it('renders nothing without saved buses', () => {
    const { container } = render(
      <FavoritesPanel favorites={[]} entries={{}} mode="live" onSelect={vi.fn()} onRemove={vi.fn()} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows each saved bus in order with its live status', () => {
    const onSelect = vi.fn()
    const onRemove = vi.fn()
    render(
      <FavoritesPanel
        favorites={['KS602', 'GONE1']}
        entries={{
          KS602: { tracking: tracking() },
          GONE1: { error: { code: 'BUS_NOT_FOUND', message: 'Bus route not found on Chalo' } },
        }}
        mode="live"
        activeCode="KS602"
        onSelect={onSelect}
        onRemove={onRemove}
      />
    )
    const cards = screen.getAllByTestId('favorite-card')
    expect(cards.map(c => c.getAttribute('data-code'))).toEqual(['KS602', 'GONE1'])
    expect(cards[0].textContent).toContain('31 km/h · next Thalassery')
    expect(cards[0].textContent).toContain('12s ago')
    expect(cards[1].getAttribute('data-status')).toBe('Not live')
    expect(screen.getByTestId('fleet-mode').getAttribute('data-mode')).toBe('live')
    expect(screen.getByLabelText('Track bus KS602 (Live)').getAttribute('aria-current')).toBe('true')

    fireEvent.click(screen.getByLabelText('Track bus GONE1 (Not live)'))
    expect(onSelect).toHaveBeenCalledWith('GONE1')
    fireEvent.click(screen.getByLabelText('Remove KS602 from My buses'))
    expect(onRemove).toHaveBeenCalledWith('KS602')
  })
})
