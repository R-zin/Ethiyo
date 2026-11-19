import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { TransitApi, ApiError } from '../services/api'
import {
  ApiErrorBody,
  BusRouteInfo,
  BusTrackingInfo,
  ConnectionMode,
  PositionFix,
} from '../types/transit'
import { haversineMeters } from '../utils/geo'
import { toMillis } from '../utils/format'

export const POLL_INTERVAL_MS = 15_000
export const MAX_TRAIL_POINTS = 120

/** The subset of EventSource the hook relies on (injectable for tests). */
export interface EventSourceLike {
  readonly readyState: number
  addEventListener(type: string, listener: (ev: MessageEvent) => void): void
  close(): void
  onerror: ((ev: Event) => void) | null
  onopen: ((ev: Event) => void) | null
}

export type EventSourceFactory = (url: string) => EventSourceLike
type Api = Pick<typeof TransitApi, 'trackBus' | 'getRoute' | 'getHistory' | 'streamUrl'>

export interface BusTrackerOptions {
  api?: Api
  /** null disables streaming (forces polling). Defaults to window.EventSource. */
  eventSourceFactory?: EventSourceFactory | null
  pollIntervalMs?: number
}

const EVENTSOURCE_CLOSED = 2

const defaultFactory: EventSourceFactory | null =
  typeof EventSource !== 'undefined' ? url => new EventSource(url) as unknown as EventSourceLike : null

function toErrorBody(err: unknown, fallbackCode: string): ApiErrorBody {
  if (err instanceof ApiError) return { code: err.code, message: err.message }
  if (err instanceof Error) return { code: fallbackCode, message: err.message }
  return { code: fallbackCode, message: 'Unexpected error' }
}

function isAbort(err: unknown): boolean {
  return err instanceof ApiError && err.code === 'ABORTED'
}

function fixFrom(info: BusTrackingInfo | null): PositionFix | null {
  const live = info?.live
  if (!live) return null
  return {
    latitude: live.latitude,
    longitude: live.longitude,
    speed: live.speed ?? 0,
    recorded_at: live.recorded_at,
  }
}

/**
 * Merges position fixes into one chronological, de-duplicated trail
 * (keyed on recorded_at), keeping the most recent MAX_TRAIL_POINTS.
 */
export function mergeTrail(...sources: PositionFix[][]): PositionFix[] {
  const byTime = new Map<number, PositionFix>()
  for (const src of sources) {
    for (const p of src) {
      const t = toMillis(p.recorded_at)
      if (t !== undefined) byTime.set(t, p)
    }
  }
  return [...byTime.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, p]) => p)
    .slice(-MAX_TRAIL_POINTS)
}

export function trailDistanceMeters(trail: PositionFix[]): number {
  let total = 0
  for (let i = 1; i < trail.length; i++) {
    total += haversineMeters(trail[i - 1].latitude, trail[i - 1].longitude, trail[i].latitude, trail[i].longitude)
  }
  return total
}

/**
 * Owns everything needed to follow one bus: the initial lookup, the route
 * (discovered once per bus — it costs a headless browser on the server), the
 * breadcrumb trail, and live updates over server-sent events with automatic
 * fallback to polling.
 *
 * Every async result is scoped to the bus code it was requested for, so
 * switching buses can never show one bus's data under another's code.
 */
export function useBusTracker(busCode: string, options: BusTrackerOptions = {}) {
  const api = options.api ?? TransitApi
  const factory = options.eventSourceFactory === undefined ? defaultFactory : options.eventSourceFactory
  const pollIntervalMs = options.pollIntervalMs ?? POLL_INTERVAL_MS

  const [tracking, setTracking] = useState<BusTrackingInfo | null>(null)
  const [error, setError] = useState<ApiErrorBody | null>(null)
  const [warning, setWarning] = useState<ApiErrorBody | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [lastUpdated, setLastUpdated] = useState<number | null>(null)
  const [trail, setTrail] = useState<PositionFix[]>([])
  const [route, setRoute] = useState<BusRouteInfo | null>(null)
  const [routeLoading, setRouteLoading] = useState(false)
  const [routeError, setRouteError] = useState<ApiErrorBody | null>(null)
  const [paused, setPaused] = useState(false)
  const [transport, setTransport] = useState<'sse' | 'polling'>(factory ? 'sse' : 'polling')
  const [mode, setMode] = useState<ConnectionMode>('connecting')
  const [loadedCode, setLoadedCode] = useState<string | null>(null)
  const [reloadNonce, setReloadNonce] = useState(0)

  // The code whose data is currently on screen; guards late responses.
  const currentCode = useRef(busCode)
  currentCode.current = busCode

  const applySnapshot = useCallback((code: string, info: BusTrackingInfo) => {
    if (currentCode.current !== code) return
    setTracking(info)
    setLastUpdated(Date.now())
    setError(null)
    setWarning(null)
    const fix = fixFrom(info)
    if (fix) setTrail(prev => mergeTrail(prev, [fix]))
  }, [])

  // Effect A: initial lookup, history and route — once per bus (or retry).
  useEffect(() => {
    setTracking(null)
    setError(null)
    setWarning(null)
    setTrail([])
    setRoute(null)
    setRouteError(null)
    setLoadedCode(null)
    setTransport(factory ? 'sse' : 'polling')
    if (!busCode) {
      setIsLoading(false)
      return
    }

    const controller = new AbortController()
    const { signal } = controller
    setIsLoading(true)

    api
      .trackBus(busCode, signal)
      .then(info => {
        if (signal.aborted) return
        applySnapshot(busCode, info)
        setLoadedCode(busCode)

        api
          .getHistory(busCode, signal)
          .then(h => {
            if (!signal.aborted) setTrail(prev => mergeTrail(h.points ?? [], prev))
          })
          .catch(() => {
            /* the trail is an enhancement; live data is already shown */
          })

        setRouteLoading(true)
        api
          .getRoute(busCode, signal)
          .then(r => {
            if (!signal.aborted) setRoute(r)
          })
          .catch(err => {
            if (!signal.aborted && !isAbort(err)) setRouteError(toErrorBody(err, 'ROUTE_DISCOVERY_FAILED'))
          })
          .finally(() => {
            if (!signal.aborted) setRouteLoading(false)
          })
      })
      .catch(err => {
        if (signal.aborted || isAbort(err)) return
        setTracking(null)
        setError(toErrorBody(err, 'TRACKING_FAILED'))
      })
      .finally(() => {
        if (!signal.aborted) setIsLoading(false)
      })

    return () => controller.abort()
    // api and factory are stable configuration and deliberately not deps.
  }, [busCode, reloadNonce, applySnapshot])

  const refresh = useCallback(async () => {
    const code = currentCode.current
    if (!code) return
    setIsRefreshing(true)
    try {
      applySnapshot(code, await api.trackBus(code))
    } catch (err) {
      // Keep the last good position on screen; surface the problem softly.
      if (currentCode.current === code && !isAbort(err)) setWarning(toErrorBody(err, 'REFRESH_FAILED'))
    } finally {
      if (currentCode.current === code) setIsRefreshing(false)
    }
  }, [api, applySnapshot])

  // Effect B: live updates once the bus is known to exist.
  const ready = loadedCode !== null && loadedCode === busCode
  useEffect(() => {
    if (!ready) return
    if (paused) {
      setMode('paused')
      return
    }

    if (transport === 'polling' || !factory) {
      setMode('polling')
      const id = setInterval(refresh, pollIntervalMs)
      return () => clearInterval(id)
    }

    let cancelled = false
    setMode('connecting')
    const es = factory(api.streamUrl(busCode))
    es.onopen = () => {
      if (!cancelled) setMode('live')
    }
    es.addEventListener('position', ev => {
      if (cancelled) return
      try {
        applySnapshot(busCode, JSON.parse(ev.data) as BusTrackingInfo)
      } catch {
        /* ignore a malformed frame; the next one will correct state */
      }
    })
    es.addEventListener('upstream_error', ev => {
      if (cancelled) return
      try {
        setWarning(JSON.parse(ev.data) as ApiErrorBody)
      } catch {
        setWarning({ code: 'UPSTREAM_ERROR', message: 'Live feed reported an upstream problem' })
      }
    })
    es.onerror = () => {
      if (cancelled) return
      if (es.readyState === EVENTSOURCE_CLOSED) {
        // The server refused the stream (e.g. capacity) or the browser gave
        // up reconnecting: degrade gracefully to polling.
        es.close()
        setTransport('polling')
      } else {
        setMode('connecting') // EventSource is retrying on its own
      }
    }

    return () => {
      cancelled = true
      es.close()
    }
  }, [ready, busCode, paused, transport, pollIntervalMs, refresh, applySnapshot])

  const retry = useCallback(() => setReloadNonce(n => n + 1), [])
  const distanceM = useMemo(() => trailDistanceMeters(trail), [trail])

  return {
    tracking,
    error,
    warning,
    isLoading,
    isRefreshing,
    lastUpdated,
    trail,
    distanceM,
    route,
    routeLoading,
    routeError,
    mode,
    paused,
    setPaused,
    refresh,
    retry,
  }
}
