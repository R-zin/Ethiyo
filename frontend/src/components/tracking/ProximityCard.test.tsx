import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ProximityCard } from './ProximityCard'

const bus = { latitude: 10.03, longitude: 76, speed: 36 }

describe('ProximityCard', () => {
  it('asks for opt-in and promises the location stays local', () => {
    const onStart = vi.fn()
    render(<ProximityCard bus={bus} status="idle" position={null} onStart={onStart} onStop={vi.fn()} />)
    expect(screen.getByText(/stays in your browser/)).toBeDefined()
    fireEvent.click(screen.getByText('Use my location'))
    expect(onStart).toHaveBeenCalledOnce()
  })

  it('shows straight-line distance and an approximate ETA', () => {
    render(
      <ProximityCard
        bus={bus}
        status="active"
        position={{ latitude: 10, longitude: 76, accuracy: 10 }}
        onStart={vi.fn()}
        onStop={vi.fn()}
      />
    )
    const text = screen.getByTestId('proximity-result').textContent
    expect(text).toContain('3.3 km')
    expect(text).toContain('straight line')
    expect(text).toContain('6 min') // 3.34 km at 36 km/h
  })

  it('does not invent an ETA for a stationary bus', () => {
    render(
      <ProximityCard
        bus={{ ...bus, speed: 0 }}
        status="active"
        position={{ latitude: 10, longitude: 76, accuracy: 10 }}
        onStart={vi.fn()}
        onStop={vi.fn()}
      />
    )
    expect(screen.getByTestId('proximity-result').textContent).toContain('no ETA yet')
  })

  it('explains a denied permission and hides itself when unsupported', () => {
    const { rerender, container } = render(
      <ProximityCard bus={bus} status="denied" position={null} onStart={vi.fn()} onStop={vi.fn()} />
    )
    expect(screen.getByText(/permission was denied/)).toBeDefined()
    rerender(<ProximityCard bus={bus} status="unsupported" position={null} onStart={vi.fn()} onStop={vi.fn()} />)
    expect(container.innerHTML).toBe('')
  })
})
