import { BusTrackingInfo, BusRouteInfo, HealthStatus } from '../types/transit'

const API_BASE = import.meta.env.VITE_API_URL || ''

class ApiError extends Error {
  code: string
  status: number

  constructor(message: string, code = 'UNKNOWN_ERROR', status = 500) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${endpoint}`
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 12000)

  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
      headers: {
        'Accept': 'application/json',
        ...(options.headers || {}),
      },
    })

    const contentType = response.headers.get('content-type') || ''
    const isJson = contentType.includes('application/json')
    const data = isJson ? await response.json() : null

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
    if (err instanceof ApiError) {
      throw err
    }
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError('Request timed out while connecting to transit service', 'TIMEOUT', 504)
    }
    const message = err instanceof Error ? err.message : 'Network error'
    throw new ApiError(message, 'NETWORK_ERROR', 0)
  } finally {
    clearTimeout(timeoutId)
  }
}

export const TransitApi = {
  async trackBus(busCode: string): Promise<BusTrackingInfo> {
    const code = busCode.trim().toUpperCase()
    return request<BusTrackingInfo>(`/api/v1/bus/${encodeURIComponent(code)}/track`)
  },

  async getRoute(busCode: string): Promise<BusRouteInfo> {
    const code = busCode.trim().toUpperCase()
    return request<BusRouteInfo>(`/api/v1/bus/${encodeURIComponent(code)}/route`)
  },

  async checkHealth(): Promise<HealthStatus> {
    const start = performance.now()
    const res = await request<HealthStatus>('/health')
    const latencyMs = Math.round(performance.now() - start)
    return { ...res, latencyMs }
  },
}
