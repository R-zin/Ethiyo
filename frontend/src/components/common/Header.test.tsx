import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Header } from './Header'
import { HealthStatus } from '../../types/transit'

function renderHeader(health: HealthStatus | null, healthLoading = false) {
  render(<Header theme="light" onToggleTheme={vi.fn()} health={health} healthLoading={healthLoading} />)
  return screen.getByTestId('api-status')
}

describe('Header status', () => {
  it('distinguishes healthy, Chalo-degraded and down', () => {
    expect(renderHeader({ message: 'ok', status: 'healthy', browser: true, latencyMs: 12 }).textContent).toContain('API Online')
  })

  it('shows when Chalo is down even though the API is up', () => {
    const status = renderHeader({
      message: 'ok',
      status: 'healthy',
      browser: true,
      degraded: true,
      upstream: { state: 'open', consecutive_failures: 5 },
    })
    expect(status.getAttribute('data-state')).toBe('degraded')
    expect(status.textContent).toContain('Chalo Degraded')
    expect(status.getAttribute('title')).toContain('circuit open')
  })

  it('shows when the API itself is down', () => {
    const status = renderHeader({ message: 'offline', status: 'error', browser: false })
    expect(status.getAttribute('data-state')).toBe('down')
    expect(status.textContent).toContain('Service Down')
  })
})
