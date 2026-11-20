import { describe, it, expect } from 'vitest'
// public/sw.js is shipped as-is (not bundled), so load its source as text
// and pull out the routing function to test the caching rules directly.
import source from '../public/sw.js?raw'

// Recorded in a plain array: the script runs once at import time, and mock
// call history would be cleared before each test.
const registered: string[] = []
const fakeSelf = {
  addEventListener: (type: string) => registered.push(type),
  location: { origin: 'https://app.example' },
}
const { strategyFor } = new Function('self', 'caches', `${source}\nreturn { strategyFor }`)(fakeSelf, {}) as {
  strategyFor: (req: { url: string; method: string; mode?: string }, origin: string) => string
}

const req = (path: string, method = 'GET', mode = 'cors') => ({ url: `https://app.example${path}`, method, mode })
const origin = 'https://app.example'

describe('service worker routing', () => {
  it('never intercepts live data', () => {
    for (const path of [
      '/api/v1/bus/KS602/track',
      '/api/v1/bus/KS602/stream',
      '/api/v1/buses/stream?codes=A,B',
      '/health',
      '/metrics',
    ]) {
      expect(strategyFor(req(path), origin)).toBe('network')
    }
  })

  it('leaves non-GET and cross-origin requests alone', () => {
    expect(strategyFor(req('/', 'POST'), origin)).toBe('network')
    expect(strategyFor({ url: 'https://a.tile.openstreetmap.org/1/2/3.png', method: 'GET' }, origin)).toBe('network')
  })

  it('serves the app shell network-first and hashed assets cache-first', () => {
    expect(strategyFor(req('/?code=KS602', 'GET', 'navigate'), origin)).toBe('network-first')
    expect(strategyFor(req('/assets/index-Bem9n-V4.js'), origin)).toBe('cache-first')
    expect(strategyFor(req('/icons/icon-192.png'), origin)).toBe('stale-while-revalidate')
  })

  it('registers its lifecycle, fetch and notification handlers', () => {
    expect(registered).toEqual(expect.arrayContaining(['install', 'activate', 'fetch', 'notificationclick']))
  })
})
