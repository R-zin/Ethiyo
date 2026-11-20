import { describe, it, expect, vi, afterEach } from 'vitest'
import { TransitApi, ApiError, TIMEOUTS } from './api'

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('TransitApi', () => {
  it('unwraps the success envelope and normalizes the bus code in the URL', async () => {
    const fetchMock = vi.fn(async () => jsonResponse(200, { success: true, data: { bus_code: 'KS602', tracking_url: 'x' } }))
    vi.stubGlobal('fetch', fetchMock)

    const res = await TransitApi.trackBus('  ks602 ')
    expect(res.bus_code).toBe('KS602')
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/bus/KS602/track', expect.anything())
  })

  it('maps structured API errors to ApiError with the server code', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      jsonResponse(404, { success: false, error: { code: 'BUS_NOT_FOUND', message: 'Bus route not found on Chalo' } })
    ))
    await expect(TransitApi.trackBus('NOPE')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'BUS_NOT_FOUND',
      status: 404,
      message: 'Bus route not found on Chalo',
    })
  })

  it('handles non-JSON error bodies', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>bad gateway</html>', { status: 502, statusText: 'Bad Gateway' })))
    await expect(TransitApi.trackBus('X')).rejects.toMatchObject({ code: 'HTTP_502', status: 502 })
  })

  it('maps network failures to NETWORK_ERROR', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    await expect(TransitApi.trackBus('X')).rejects.toMatchObject({ code: 'NETWORK_ERROR', status: 0 })
  })

  it('gives route discovery a longer timeout than live lookups', async () => {
    vi.useFakeTimers()
    const hang = (_url: string, init?: RequestInit) =>
      new Promise<Response>((_, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })
    vi.stubGlobal('fetch', vi.fn(hang))

    const route = TransitApi.getRoute('X').catch(e => e)
    await vi.advanceTimersByTimeAsync(TIMEOUTS.default + 1)
    // Still waiting: the server may legitimately take this long to drive a browser.
    let settled = false
    route.then(() => (settled = true))
    await Promise.resolve()
    expect(settled).toBe(false)

    await vi.advanceTimersByTimeAsync(TIMEOUTS.route - TIMEOUTS.default)
    const err = await route
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('TIMEOUT')
  })

  it('distinguishes a caller abort from a timeout', async () => {
    vi.stubGlobal('fetch', vi.fn((_url: string, init?: RequestInit) =>
      new Promise<Response>((_, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })
    ))
    const controller = new AbortController()
    const p = TransitApi.trackBus('X', controller.signal)
    controller.abort()
    await expect(p).rejects.toMatchObject({ code: 'ABORTED' })
  })

  it('builds stream and history URLs', async () => {
    expect(TransitApi.streamUrl('ks 602')).toBe('/api/v1/bus/KS%20602/stream')
    const fetchMock = vi.fn(async () => jsonResponse(200, { success: true, data: { bus_code: 'A', points: [], distance_m: 0 } }))
    vi.stubGlobal('fetch', fetchMock)
    await TransitApi.getHistory('a')
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/bus/A/history', expect.anything())
  })
})

describe('fleet and export URLs', () => {
  it('requests several buses in one call', async () => {
    const fetchMock = vi.fn(async () => jsonResponse(200, { success: true, data: { buses: [] } }))
    vi.stubGlobal('fetch', fetchMock)
    await TransitApi.trackMany([' ks602', 'dl1pc0001 '])
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/buses/track?codes=KS602,DL1PC0001', expect.anything())
  })

  it('builds the multi-bus stream and trail export URLs', () => {
    expect(TransitApi.fleetStreamUrl(['a1', 'B2'])).toBe('/api/v1/buses/stream?codes=A1,B2')
    expect(TransitApi.trailExportUrl('ks602', 'gpx')).toBe('/api/v1/bus/KS602/history?format=gpx')
    expect(TransitApi.trailExportUrl('ks602', 'geojson')).toBe('/api/v1/bus/KS602/history?format=geojson')
  })
})
