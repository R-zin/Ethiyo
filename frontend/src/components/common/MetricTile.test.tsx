import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MetricTile } from './MetricTile'

describe('MetricTile', () => {
  it('renders label, value and unit properly', () => {
    render(<MetricTile label="Speed" value="45" unit="km/h" subtext="In Transit" />)
    expect(screen.getByText('Speed')).toBeDefined()
    expect(screen.getByText('45')).toBeDefined()
    expect(screen.getByText('km/h')).toBeDefined()
    expect(screen.getByText('In Transit')).toBeDefined()
  })
})
