import { useEffect, useState } from 'react'
import { TransitApi } from '../services/api'
import { ApiErrorBody, BusErrorEvent, BusTrackingInfo, ConnectionMode, FleetItem } from '../types/transit'
import { EventSourceFactory, EventSourceLike } from './useBusTracker'

export const FLEET_POLL_INTERVAL_MS = 30_000
const EVENTSOURCE_CLOSED = 2

export interface FleetEntry {
  tracking?: BusTrackingInfo
  error?: ApiErrorBody
}

type Api = Pick<typeof TransitApi, 'trackMany' | 'fleetStreamUrl'>

export interface FleetOptions {
  api?: Api
  eventSourceFactory?: EventSourceFactory | null
  pollIntervalMs?: number
}

const defaultFactory: EventSourceFactory | null =
  typeof EventSource !== 'undefined' ? url => new EventSource(url) as unknown as EventSourceLike : null

/**
 * Live status for several buses ("My buses") over ONE server-sent events
 * connection, with an initial batch lookup and a polling fallback. One
 * EventSource per bus would exhaust the browser's ~6 connections per origin.
 */
export function useFleet(codes: string[], options: FleetOptions = {}) {
  const api = options.api ?? TransitApi
  const factory = options.eventSourceFactory === undefined ? defaultFactory : options.eventSourceFactory
  const pollIntervalMs = options.pollIntervalMs ?? FLEET_POLL_INTERVAL_MS

  const [entries, setEntries] = useState<Record<string, FleetEntry>>({})
  const [mode, setMode] = useState<ConnectionMode>('connecting')
  const [isLoading, setIsLoading] = useState(false)

  // A stable key: the effect restarts only when the set of buses changes.
  const key = codes.join(',')

  useEffect(() => {
    const list = key ? key.split(',') : []
    // Forget buses that were removed; keep the rest on screen (no flicker).
    setEntries(prev => Object.fromEntries(Object.entries(prev).filter(([code]) => list.includes(code))))
    if (list.length === 0) {
      setIsLoading(false)
      return
    }

    let cancelled = false
    const controller = new AbortController()
    let es: EventSourceLike | null = null
    let pollId: ReturnType<typeof setInterval> | null = null

    const apply = (items: FleetItem[]) => {
      if (cancelled) return
      setEntries(prev => {
        const next = { ...prev }
        for (const item of items) {
          if (!list.includes(item.bus_code)) continue
          next[item.bus_code] = item.tracking ? { tracking: item.tracking } : { error: item.error }
        }
        return next
      })
    }

    const poll = () => {
      api
        .trackMany(list)
        .then(res => apply(res.buses))
        .catch(() => {
          /* keep the last known statuses on screen */
        })
    }

    const startPolling = () => {
      if (cancelled || pollId !== null) return
      setMode('polling')
      pollId = setInterval(poll, pollIntervalMs)
    }

    setIsLoading(true)
    setMode('connecting')
    api
      .trackMany(list, controller.signal)
      .then(res => apply(res.buses))
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setIsLoading(false)
      })

    if (factory) {
      const source = factory(api.fleetStreamUrl(list))
      es = source
      source.onopen = () => {
        if (!cancelled) setMode('live')
      }
      source.addEventListener('position', ev => {
        try {
          const info = JSON.parse(ev.data) as BusTrackingInfo
          apply([{ bus_code: info.bus_code, tracking: info }])
        } catch {
          /* ignore malformed frames */
        }
      })
      source.addEventListener('bus_error', ev => {
        try {
          const err = JSON.parse(ev.data) as BusErrorEvent
          apply([{ bus_code: err.bus_code, error: { code: err.code, message: err.message } }])
        } catch {
          /* ignore malformed frames */
        }
      })
      source.onerror = () => {
        if (cancelled) return
        if (source.readyState === EVENTSOURCE_CLOSED) {
          source.close()
          es = null
          startPolling()
        } else {
          setMode('connecting')
        }
      }
    } else {
      startPolling()
    }

    return () => {
      cancelled = true
      controller.abort()
      es?.close()
      if (pollId !== null) clearInterval(pollId)
    }
    // api/factory are stable configuration; the bus list is captured by `key`.
  }, [key, pollIntervalMs])

  return { entries, mode, isLoading }
}
