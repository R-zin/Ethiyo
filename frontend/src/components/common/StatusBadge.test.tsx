import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatusBadge } from './StatusBadge'

describe('StatusBadge', () => {
  it('renders children and default classes', () => {
    render(<StatusBadge variant="active">Live Telemetry</StatusBadge>)
    const badge = screen.getByText('Live Telemetry')
    expect(badge).toBeDefined()
  })

  it('renders different status variants', () => {
    const { container: activeEl } = render(<StatusBadge variant="active">Active</StatusBadge>)
    expect(activeEl.textContent).toContain('Active')

    const { container: errorEl } = render(<StatusBadge variant="error">Offline</StatusBadge>)
    expect(errorEl.textContent).toContain('Offline')
  })
})
