import { BusTrackingInfo, BusRouteInfo, BusHistory, HealthStatus } from '../types/transit'

const API_BASE = import.meta.env.VITE_API_URL || ''

/**
 * Client-side timeouts are sized to the server's: live lookups are a single
 * upstream HTTP call (REQUEST_TIMEOUT, 15s default), while route discovery
 * may wait for a browser slot and then drive a headless browser (up to
 * 2 × BROWSER_TIMEOUT). Aborting earlier would discard answers the server is
 * still about to deliver.
 */
export const TIMEOUTS = {
  default: 20_000,
  route: 45_000,
  health: 8_000,
} as const

export class ApiError extends Error {
  code: string
  status: number

  constructor(message: string, code = 'UNKNOWN_ERROR', status = 500) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

interface RequestOptions extends RequestInit {
  timeoutMs?: number
}

async function request<T>(endpoint: string, { timeoutMs = TIMEOUTS.default, ...options }: RequestOptions = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)
  // Honour a caller-provided signal too (e.g. a superseded search).
  const external = options.signal
  const onExternalAbort = () => controller.abort()
  external?.addEventListener('abort', onExternalAbort)

  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
      headers: {
        Accept: 'application/json',
        ...(options.headers || {}),
      },
    })

    const contentType = response.headers.get('content-type') || ''
    const data = contentType.includes('application/json') ? await response.json() : null

    if (!response.ok) {
      const code = data?.error?.code || `HTTP_${response.status}`
      const message = data?.error?.message || response.statusText || 'Request failed'
      throw new ApiError(message, code, response.status)
    }

    if (data && typeof data === 'object' && 'data' in data) {
      return data.data as T
    }
    return (data || {}) as T
  } catch (err: unknown) {
    if (err instanceof ApiError) throw err
    if (err instanceof DOMException && err.name === 'AbortError') {
      if (external?.aborted) throw new ApiError('Request was superseded', 'ABORTED', 0)
      throw new ApiError('Request timed out while connecting to transit service', 'TIMEOUT', 504)
    }
    const message = err instanceof Error ? err.message : 'Network error'
    throw new ApiError(message, 'NETWORK_ERROR', 0)
  } finally {
    clearTimeout(timeoutId)
    external?.removeEventListener('abort', onExternalAbort)
  }
}

const busPath = (busCode: string, leaf: string) =>
  `/api/v1/bus/${encodeURIComponent(busCode.trim().toUpperCase())}/${leaf}`

export const TransitApi = {
  trackBus(busCode: string, signal?: AbortSignal): Promise<BusTrackingInfo> {
    return request<BusTrackingInfo>(busPath(busCode, 'track'), { signal })
  },

  getRoute(busCode: string, signal?: AbortSignal): Promise<BusRouteInfo> {
    return request<BusRouteInfo>(busPath(busCode, 'route'), { signal, timeoutMs: TIMEOUTS.route })
  },

  getHistory(busCode: string, signal?: AbortSignal): Promise<BusHistory> {
    return request<BusHistory>(busPath(busCode, 'history'), { signal })
  },

  /** URL of the server-sent events stream for live position updates. */
  streamUrl(busCode: string): string {
    return `${API_BASE}${busPath(busCode, 'stream')}`
  },

  async checkHealth(): Promise<HealthStatus> {
    const start = performance.now()
    const res = await request<HealthStatus>('/health', { timeoutMs: TIMEOUTS.health })
    const latencyMs = Math.round(performance.now() - start)
    return { ...res, latencyMs }
  },
}
