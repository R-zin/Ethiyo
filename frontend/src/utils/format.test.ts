import { describe, it, expect } from 'vitest'
import { formatSpeed, formatCoordinates, formatRelativeTime } from './format'

describe('format utilities', () => {
  it('formats speed values cleanly', () => {
    expect(formatSpeed(24.6)).toBe('25 km/h')
    expect(formatSpeed(0)).toBe('0 km/h')
    expect(formatSpeed(undefined)).toBe('0 km/h')
  })

  it('formats coordinates with cardinal directions', () => {
    expect(formatCoordinates(12.9716, 77.5946)).toBe('12.9716° N, 77.5946° E')
    expect(formatCoordinates(-33.8688, 151.2093)).toBe('33.8688° S, 151.2093° E')
  })

  it('formats relative elapsed time accurately', () => {
    expect(formatRelativeTime(undefined)).toBe('Live now')
    const now = Date.now()
    expect(formatRelativeTime(now - 2000)).toBe('Just now')
    expect(formatRelativeTime(now - 15000)).toBe('15s ago')
    expect(formatRelativeTime(now - 120000)).toBe('2m ago')
  })
})
