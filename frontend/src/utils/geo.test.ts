import { describe, it, expect } from 'vitest'
import { haversineMeters, compassPoint, formatDistance, estimateEtaMinutes } from './geo'

describe('geo utilities', () => {
  it('computes great-circle distances', () => {
    expect(haversineMeters(12.97, 77.59, 12.97, 77.59)).toBe(0)
    expect(haversineMeters(0, 0, 1, 0)).toBeCloseTo(111195, -1)
    // Agrees with the Go backend's implementation (same Earth radius).
    expect(haversineMeters(10, 76, 10.01, 76)).toBeCloseTo(1111.95, 0)
  })

  it('maps headings to compass points, wrapping correctly', () => {
    expect(compassPoint(0)).toBe('N')
    expect(compassPoint(44)).toBe('NE')
    expect(compassPoint(90)).toBe('E')
    expect(compassPoint(200)).toBe('S')
    expect(compassPoint(337.6)).toBe('N')
    expect(compassPoint(-90)).toBe('W')
    expect(compassPoint(720 + 135)).toBe('SE')
  })

  it('formats distances for humans', () => {
    expect(formatDistance(42.4)).toBe('42 m')
    expect(formatDistance(1530)).toBe('1.5 km')
    expect(formatDistance(23_400)).toBe('23 km')
    expect(formatDistance(NaN)).toBe('—')
    expect(formatDistance(-1)).toBe('—')
  })

  it('estimates a naive ETA only for moving buses', () => {
    expect(estimateEtaMinutes(3000, 36)).toBe(5) // 3 km at 36 km/h = 5 min
    expect(estimateEtaMinutes(50, 40)).toBe(1) // never "0 min"
    expect(estimateEtaMinutes(3000, 0)).toBeNull()
    expect(estimateEtaMinutes(3000, 2)).toBeNull() // crawling ≈ stopped
    expect(estimateEtaMinutes(3000, undefined)).toBeNull()
  })
})
