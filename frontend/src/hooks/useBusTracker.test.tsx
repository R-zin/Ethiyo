import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useBusTracker, mergeTrail, trailDistanceMeters, MAX_TRAIL_POINTS } from './useBusTracker'
import { FakeEventSource, fakeFactory } from '../test/fakeEventSource'
import { ApiError } from '../services/api'
import { BusTrackingInfo, PositionFix } from '../types/transit'

function snapshot(code: string, lat = 10, recordedSec = 1000, extra: Partial<BusTrackingInfo['live']> = {}): BusTrackingInfo {
  return {
    bus_code: code,
    tracking_url: `https://chalo.com/raw?vehicleNo=${code}`,
    live: {
      bus_code: code,
      latitude: lat,
      longitude: 76,
      speed: 25,
      recorded_at: new Date(recordedSec * 1000).toISOString(),
      live_tracking: true,
      tracking_url: 't',
      age_seconds: 3,
      stale: false,
      ...extra,
    },
  }
}

const factory = fakeFactory
const latest = () => FakeEventSource.latest()

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function makeApi() {
  return {
    trackBus: vi.fn(async (code: string) => snapshot(code)),
    getRoute: vi.fn(async (code: string) => ({ bus_code: code, tracking_url: 't', route_url: `route-${code}` })),
    getHistory: vi.fn(async (code: string) => ({
      bus_code: code,
      points: [{ latitude: 9.99, longitude: 76, speed: 20, recorded_at: new Date(970_000).toISOString() }],
      distance_m: 0,
    })),
    streamUrl: (code: string) => `/api/v1/bus/${code}/stream`,
  }
}

beforeEach(() => {
  FakeEventSource.reset()
})

describe('useBusTracker', () => {
  it('loads the bus, its route and history, then goes live over SSE', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))

    await waitFor(() => expect(result.current.tracking?.bus_code).toBe('KS602'))
    await waitFor(() => expect(result.current.route?.route_url).toBe('route-KS602'))
    await waitFor(() => expect(result.current.trail).toHaveLength(2)) // history point + current fix

    const es = latest()
    expect(es.url).toBe('/api/v1/bus/KS602/stream')
    expect(result.current.mode).toBe('connecting')
    act(() => es.open())
    expect(result.current.mode).toBe('live')

    act(() => es.emit('position', snapshot('KS602', 10.01, 1030, { heading: 0 })))
    expect(result.current.tracking?.live?.latitude).toBe(10.01)
    expect(result.current.tracking?.live?.heading).toBe(0)
    expect(result.current.trail).toHaveLength(3)
    expect(result.current.distanceM).toBeGreaterThan(1000)
  })

  it('never shows a superseded bus when responses arrive out of order', async () => {
    const api = makeApi()
    const slowA = deferred<BusTrackingInfo>()
    api.trackBus.mockImplementation(async (code: string) => (code === 'AAA1' ? slowA.promise : snapshot(code)))

    const { result, rerender } = renderHook(({ code }) => useBusTracker(code, { api, eventSourceFactory: factory }), {
      initialProps: { code: 'AAA1' },
    })
    rerender({ code: 'BBB2' })
    await waitFor(() => expect(result.current.tracking?.bus_code).toBe('BBB2'))

    // The slow, superseded response for AAA1 lands last.
    await act(async () => slowA.resolve(snapshot('AAA1', 50)))
    expect(result.current.tracking?.bus_code).toBe('BBB2')
    expect(result.current.tracking?.live?.latitude).toBe(10)
    await waitFor(() => expect(result.current.route?.route_url).toBe('route-BBB2'))
    expect(api.getRoute).not.toHaveBeenCalledWith('AAA1', expect.anything())
  })

  it('clears the previous bus route and trail when switching buses', async () => {
    const api = makeApi()
    const routeB = deferred<{ bus_code: string; tracking_url: string; route_url: string }>()
    api.getRoute.mockImplementation(async (code: string) =>
      code === 'BBB2' ? routeB.promise : { bus_code: code, tracking_url: 't', route_url: `route-${code}` }
    )
    const { result, rerender } = renderHook(({ code }) => useBusTracker(code, { api, eventSourceFactory: factory }), {
      initialProps: { code: 'AAA1' },
    })
    await waitFor(() => expect(result.current.route?.route_url).toBe('route-AAA1'))

    rerender({ code: 'BBB2' })
    await waitFor(() => expect(result.current.tracking?.bus_code).toBe('BBB2'))
    expect(result.current.route).toBeNull() // not AAA1's route
    expect(result.current.routeLoading).toBe(true)
    expect(result.current.trail.every(p => p.latitude !== 50)).toBe(true)
  })

  it('discovers the route once per bus, not on every refresh', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))
    await waitFor(() => expect(result.current.route).not.toBeNull())

    for (let i = 0; i < 3; i++) {
      await act(() => result.current.refresh())
    }
    expect(api.trackBus).toHaveBeenCalledTimes(4)
    expect(api.getRoute).toHaveBeenCalledTimes(1)
  })

  it('reports a fatal error for an unknown bus without opening a stream', async () => {
    const api = makeApi()
    api.trackBus.mockRejectedValue(new ApiError('Bus route not found on Chalo', 'BUS_NOT_FOUND', 404))
    const { result } = renderHook(() => useBusTracker('NOPE1', { api, eventSourceFactory: factory }))

    await waitFor(() => expect(result.current.error?.code).toBe('BUS_NOT_FOUND'))
    expect(result.current.tracking).toBeNull()
    expect(result.current.isLoading).toBe(false)
    expect(FakeEventSource.instances).toHaveLength(0)
    expect(api.getRoute).not.toHaveBeenCalled()
  })

  it('retry() reloads after a failure', async () => {
    const api = makeApi()
    api.trackBus.mockRejectedValueOnce(new ApiError('down', 'NETWORK_ERROR', 0))
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))
    await waitFor(() => expect(result.current.error?.code).toBe('NETWORK_ERROR'))

    act(() => result.current.retry())
    await waitFor(() => expect(result.current.tracking?.bus_code).toBe('KS602'))
    expect(result.current.error).toBeNull()
  })

  it('falls back to polling when the stream is closed for good', async () => {
    const api = makeApi()
    const { result } = renderHook(() =>
      useBusTracker('KS602', { api, eventSourceFactory: factory, pollIntervalMs: 30 })
    )
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))

    // A transient drop: EventSource retries by itself, no fallback.
    act(() => latest().fail(false))
    expect(result.current.mode).toBe('connecting')
    expect(FakeEventSource.instances).toHaveLength(1)

    act(() => latest().fail(true))
    await waitFor(() => expect(result.current.mode).toBe('polling'))
    expect(latest().closed).toBe(true)

    const callsBefore = api.trackBus.mock.calls.length
    await waitFor(() => expect(api.trackBus.mock.calls.length).toBeGreaterThan(callsBefore + 1))
  })

  it('polls directly when EventSource is unavailable', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: null, pollIntervalMs: 30 }))
    await waitFor(() => expect(result.current.mode).toBe('polling'))
    await waitFor(() => expect(api.trackBus.mock.calls.length).toBeGreaterThan(2))
  })

  it('surfaces upstream errors as a soft warning and clears it on recovery', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))

    act(() => latest().emit('upstream_error', { code: 'UPSTREAM_CHALO_ERROR', message: 'Upstream Chalo service failure' }))
    expect(result.current.warning?.code).toBe('UPSTREAM_CHALO_ERROR')
    expect(result.current.tracking).not.toBeNull() // last known position is kept

    act(() => latest().emit('position', snapshot('KS602', 10.02, 1060)))
    expect(result.current.warning).toBeNull()
  })

  it('pausing closes the stream and resuming reopens it', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))

    act(() => result.current.setPaused(true))
    expect(result.current.mode).toBe('paused')
    expect(FakeEventSource.instances[0].closed).toBe(true)

    act(() => result.current.setPaused(false))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2))
    expect(latest().closed).toBe(false)
  })

  it('closes the stream on unmount', async () => {
    const api = makeApi()
    const { unmount } = renderHook(() => useBusTracker('KS602', { api, eventSourceFactory: factory }))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    unmount()
    expect(latest().closed).toBe(true)
  })

  it('does nothing for an empty code', () => {
    const api = makeApi()
    const { result } = renderHook(() => useBusTracker('', { api, eventSourceFactory: factory }))
    expect(result.current.tracking).toBeNull()
    expect(result.current.isLoading).toBe(false)
    expect(api.trackBus).not.toHaveBeenCalled()
  })
})

describe('trail helpers', () => {
  const fix = (sec: number, lat = 10): PositionFix => ({
    latitude: lat,
    longitude: 76,
    speed: 0,
    recorded_at: new Date(sec * 1000).toISOString(),
  })

  it('merges chronologically and de-duplicates by recorded_at', () => {
    const merged = mergeTrail([fix(3), fix(1)], [fix(2), fix(3, 99)])
    expect(merged.map(p => Date.parse(p.recorded_at) / 1000)).toEqual([1, 2, 3])
    expect(merged[2].latitude).toBe(99) // later source wins for the same timestamp
  })

  it('caps the trail length', () => {
    const many = Array.from({ length: MAX_TRAIL_POINTS + 30 }, (_, i) => fix(i))
    const merged = mergeTrail(many)
    expect(merged).toHaveLength(MAX_TRAIL_POINTS)
    expect(Date.parse(merged[0].recorded_at) / 1000).toBe(30) // oldest dropped
  })

  it('ignores fixes with invalid timestamps', () => {
    expect(mergeTrail([{ ...fix(1), recorded_at: 'not-a-date' }, fix(2)])).toHaveLength(1)
  })

  it('sums distance along the trail', () => {
    expect(trailDistanceMeters([])).toBe(0)
    const d = trailDistanceMeters([fix(1, 10), fix(2, 10.01), fix(3, 10.02)])
    expect(d).toBeGreaterThan(2200)
    expect(d).toBeLessThan(2250)
  })
})
