import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AlertsCard, AlertBanners } from './AlertsCard'

function renderCard(over: Partial<Parameters<typeof AlertsCard>[0]> = {}) {
  const props = {
    busCode: 'KS602',
    config: { approachMinutes: null, offline: false },
    permission: 'default' as const,
    locationStatus: 'idle' as const,
    onApproachChange: vi.fn(),
    onOfflineChange: vi.fn(),
    onEnableLocation: vi.fn(),
    ...over,
  }
  render(<AlertsCard {...props} />)
  return props
}

describe('AlertsCard', () => {
  it('arming the arrival alert also asks for location', () => {
    const props = renderCard()
    fireEvent.change(screen.getByLabelText(/Notify me when it's/), { target: { value: '5' } })
    expect(props.onApproachChange).toHaveBeenCalledWith(5)
    expect(props.onEnableLocation).toHaveBeenCalledOnce()
  })

  it('does not re-request location that is already on', () => {
    const props = renderCard({ locationStatus: 'active' })
    fireEvent.change(screen.getByLabelText(/Notify me when it's/), { target: { value: '10' } })
    expect(props.onEnableLocation).not.toHaveBeenCalled()
  })

  it('turning the arrival alert off passes null', () => {
    const props = renderCard({ config: { approachMinutes: 5, offline: false }, locationStatus: 'active' })
    fireEvent.change(screen.getByLabelText(/Notify me when it's/), { target: { value: 'off' } })
    expect(props.onApproachChange).toHaveBeenCalledWith(null)
  })

  it('explains when the arrival alert is waiting for location', () => {
    renderCard({ config: { approachMinutes: 5, offline: false }, locationStatus: 'denied' })
    expect(screen.getByTestId('alerts-need-location')).toBeDefined()
  })

  it('toggles the stopped-reporting alert and shows notification status', () => {
    const props = renderCard({ config: { approachMinutes: null, offline: true }, permission: 'denied' })
    fireEvent.click(screen.getByLabelText('Notify me if it stops reporting'))
    expect(props.onOfflineChange).toHaveBeenCalledWith(false)
    expect(screen.getByText(/notifications are blocked/)).toBeDefined()
  })
})

describe('AlertBanners', () => {
  it('renders fired alerts and lets the user dismiss them', () => {
    const onDismiss = vi.fn()
    render(
      <AlertBanners
        alerts={[{ id: 7, busCode: 'KS602', kind: 'approach', title: 'Bus KS602 is almost here', body: 'About 3 min away.' }]}
        onDismiss={onDismiss}
      />
    )
    expect(screen.getByRole('alert').textContent).toContain('Bus KS602 is almost here')
    fireEvent.click(screen.getByLabelText('Dismiss alert'))
    expect(onDismiss).toHaveBeenCalledWith(7)
  })
})
