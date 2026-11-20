import { describe, it, expect } from 'vitest'
import { buildPopupContent, busIconHtml } from './mapMarkup'

describe('map popup', () => {
  // Regression: route_name comes from Chalo and used to be interpolated into
  // an HTML string passed to Leaflet's bindPopup — a stored XSS vector.
  it('renders hostile upstream strings as inert text', () => {
    const payload = '<img src=x onerror="window.__pwned=1">'
    const node = buildPopupContent({
      busCode: 'KS602<script>alert(1)</script>',
      routeName: payload,
      speedKmh: 31.6,
      latitude: 11.2059,
      longitude: 75.81147,
    })
    expect(node.querySelector('img')).toBeNull()
    expect(node.querySelector('script')).toBeNull()
    expect(node.textContent).toContain(payload)
    expect(node.textContent).toContain('32 km/h')
    expect(node.textContent).toContain('11.2059, 75.8115')
  })

  it('flags stale positions and falls back for a missing route name', () => {
    const node = buildPopupContent({ busCode: 'A', latitude: 1, longitude: 2, stale: true })
    expect(node.textContent).toContain('Live Transit Route')
    expect(node.textContent).toContain('outdated')
  })
})

describe('bus marker icon', () => {
  it('rotates the heading arrow to the direction of travel', () => {
    const html = busIconHtml(135)
    expect(html).toContain('data-heading="135"')
    expect(html).toContain('rotate(135deg)')
  })

  it('normalizes headings and omits the arrow when unknown', () => {
    expect(busIconHtml(-90)).toContain('data-heading="270"')
    expect(busIconHtml(undefined)).not.toContain('bus-heading')
    expect(busIconHtml(NaN)).not.toContain('bus-heading')
  })

  it('drops the live pulse for stale buses', () => {
    expect(busIconHtml(0, false)).toContain('animate-ping')
    expect(busIconHtml(0, true)).not.toContain('animate-ping')
  })
})
