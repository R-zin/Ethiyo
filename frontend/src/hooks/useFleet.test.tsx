import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useFleet } from './useFleet'
import { FakeEventSource, fakeFactory } from '../test/fakeEventSource'
import { BusTrackingInfo, FleetResponse } from '../types/transit'

const tracking = (code: string, lat = 10): BusTrackingInfo => ({
  bus_code: code,
  tracking_url: 't',
  live: {
    bus_code: code,
    latitude: lat,
    longitude: 76,
    recorded_at: new Date().toISOString(),
    live_tracking: true,
    tracking_url: 't',
    age_seconds: 1,
    stale: false,
  },
})

function makeApi() {
  return {
    trackMany: vi.fn(
      async (codes: string[]): Promise<FleetResponse> => ({
        buses: codes.map(c =>
          c === 'GONE1'
            ? { bus_code: c, error: { code: 'BUS_NOT_FOUND', message: 'Bus route not found on Chalo' } }
            : { bus_code: c, tracking: tracking(c) }
        ),
      })
    ),
    fleetStreamUrl: (codes: string[]) => `/api/v1/buses/stream?codes=${codes.join(',')}`,
  }
}

beforeEach(() => FakeEventSource.reset())

describe('useFleet', () => {
  it('loads all saved buses, then follows them over one stream', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useFleet(['A1', 'GONE1'], { api, eventSourceFactory: fakeFactory }))

    await waitFor(() => expect(result.current.entries.A1?.tracking?.bus_code).toBe('A1'))
    expect(result.current.entries.GONE1?.error?.code).toBe('BUS_NOT_FOUND')
    expect(FakeEventSource.instances).toHaveLength(1)
    expect(FakeEventSource.latest().url).toBe('/api/v1/buses/stream?codes=A1,GONE1')

    act(() => FakeEventSource.latest().open())
    expect(result.current.mode).toBe('live')

    act(() => FakeEventSource.latest().emit('position', tracking('A1', 10.5)))
    expect(result.current.entries.A1?.tracking?.live?.latitude).toBe(10.5)

    act(() => FakeEventSource.latest().emit('bus_error', { bus_code: 'A1', code: 'UPSTREAM_UNAVAILABLE', message: 'down' }))
    expect(result.current.entries.A1?.error?.code).toBe('UPSTREAM_UNAVAILABLE')
    expect(result.current.entries.A1?.tracking).toBeUndefined()
  })

  it('ignores events for buses it is not following', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useFleet(['A1'], { api, eventSourceFactory: fakeFactory }))
    await waitFor(() => expect(result.current.entries.A1).toBeDefined())
    act(() => FakeEventSource.latest().emit('position', tracking('INTRUDER')))
    expect(result.current.entries.INTRUDER).toBeUndefined()
  })

  it('reconnects with the new list when buses are added or removed', async () => {
    const api = makeApi()
    const { result, rerender } = renderHook(({ codes }) => useFleet(codes, { api, eventSourceFactory: fakeFactory }), {
      initialProps: { codes: ['A1', 'B2'] },
    })
    await waitFor(() => expect(result.current.entries.B2).toBeDefined())
    const first = FakeEventSource.latest()

    rerender({ codes: ['A1'] })
    expect(first.closed).toBe(true)
    expect(FakeEventSource.latest().url).toBe('/api/v1/buses/stream?codes=A1')
    expect(result.current.entries.B2).toBeUndefined() // forgotten
    expect(result.current.entries.A1).toBeDefined() // kept: no flicker

    // Same list, new array identity: no reconnect.
    const count = FakeEventSource.instances.length
    rerender({ codes: ['A1'] })
    expect(FakeEventSource.instances).toHaveLength(count)
  })

  it('does nothing without saved buses', () => {
    const api = makeApi()
    const { result } = renderHook(() => useFleet([], { api, eventSourceFactory: fakeFactory }))
    expect(api.trackMany).not.toHaveBeenCalled()
    expect(FakeEventSource.instances).toHaveLength(0)
    expect(result.current.entries).toEqual({})
  })

  it('falls back to polling when the stream is refused', async () => {
    const api = makeApi()
    const { result } = renderHook(() => useFleet(['A1'], { api, eventSourceFactory: fakeFactory, pollIntervalMs: 25 }))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    act(() => FakeEventSource.latest().fail(true))
    expect(result.current.mode).toBe('polling')
    expect(FakeEventSource.latest().closed).toBe(true)
    await waitFor(() => expect(api.trackMany.mock.calls.length).toBeGreaterThan(2))
  })

  it('polls when EventSource is unavailable and stops on unmount', async () => {
    const api = makeApi()
    const { result, unmount } = renderHook(() => useFleet(['A1'], { api, eventSourceFactory: null, pollIntervalMs: 25 }))
    expect(result.current.mode).toBe('polling')
    await waitFor(() => expect(api.trackMany.mock.calls.length).toBeGreaterThan(1))
    unmount()
    const calls = api.trackMany.mock.calls.length
    await new Promise(r => setTimeout(r, 80))
    expect(api.trackMany.mock.calls.length).toBe(calls)
  })
})
