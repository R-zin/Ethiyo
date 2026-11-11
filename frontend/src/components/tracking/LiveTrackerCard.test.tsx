import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { LiveTrackerCard } from './LiveTrackerCard'

describe('LiveTrackerCard', () => {
  const sampleData = {
    bus_code: 'KS602',
    tracking_url: 'https://chalo.com/track/ks602',
    position: {
      latitude: 12.9716,
      longitude: 77.5946,
      speed_kmh: 40,
      route_name: 'Airport Express',
      next_stop: 'Terminal 1',
      prev_stop: 'Hebbal',
    },
  }

  it('renders bus telemetry and aria-pressed on auto-refresh toggle', () => {
    const handleRefresh = vi.fn()
    render(<LiveTrackerCard data={sampleData} onRefresh={handleRefresh} />)

    expect(screen.getByText('KS602')).toBeDefined()
    expect(screen.getByText('Airport Express')).toBeDefined()
    expect(screen.getAllByText('Terminal 1').length).toBeGreaterThan(0)

    const toggleBtn = screen.getByRole('button', { name: /Toggle auto refresh polling/i })
    expect(toggleBtn).toBeDefined()
    expect(toggleBtn.getAttribute('aria-pressed')).toBe('true')

    fireEvent.click(toggleBtn)
    expect(toggleBtn.getAttribute('aria-pressed')).toBe('false')
  })

  it('pauses polling when document.hidden is true', () => {
    vi.useFakeTimers()
    const handleRefresh = vi.fn()
    render(<LiveTrackerCard data={sampleData} onRefresh={handleRefresh} />)

    // Simulate switching tabs
    Object.defineProperty(document, 'hidden', { value: true, configurable: true })
    fireEvent(document, new Event('visibilitychange'))

    // Advance 30 seconds
    act(() => {
      vi.advanceTimersByTime(30000)
    })

    // Polling was paused, so handleRefresh was not triggered by timer
    expect(handleRefresh).not.toHaveBeenCalled()

    // Tab becomes visible again
    Object.defineProperty(document, 'hidden', { value: false, configurable: true })
    fireEvent(document, new Event('visibilitychange'))

    vi.useRealTimers()
  })

  it('displays backoff interval when errors occur', () => {
    const handleRefresh = vi.fn()
    const { rerender } = render(
      <LiveTrackerCard data={sampleData} onRefresh={handleRefresh} hasError={false} />
    )

    rerender(
      <LiveTrackerCard data={sampleData} onRefresh={handleRefresh} hasError={true} />
    )

    // First backoff step is 30s
    expect(screen.getByText(/30s/)).toBeDefined()
  })
})
