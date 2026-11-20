// These types mirror the Go API's JSON exactly (internal/models). Keep them
// in sync: a drift here silently blanks the UI rather than failing loudly.

export interface BusLiveInfo {
  bus_code: string
  vehicle_code?: string
  operator?: string
  route_name?: string
  latitude: number
  longitude: number
  /** km/h; omitted by the API when the bus is stationary */
  speed?: number
  /** RFC 3339 timestamp of when the vehicle recorded this position */
  recorded_at: string
  next_stop?: string
  previous_stop?: string
  live_tracking: boolean
  tracking_url: string
  /** Direction of travel, degrees clockwise from north. Absent until the bus has moved. */
  heading?: number
  /** Seconds between the vehicle recording the fix and the server answering */
  age_seconds: number
  /** True when the position is too old to be trusted as live */
  stale: boolean
}

export interface BusTrackingInfo {
  bus_code: string
  tracking_url: string
  live?: BusLiveInfo
  /** True when Chalo is unreachable and `live` is the last known position */
  degraded?: boolean
}

export interface BusRouteInfo {
  bus_code: string
  tracking_url: string
  route_url: string
}

export interface PositionFix {
  latitude: number
  longitude: number
  speed: number
  recorded_at: string
}

export interface BusHistory {
  bus_code: string
  points: PositionFix[]
  distance_m: number
}

export interface ApiErrorBody {
  code: string
  message: string
}

/** Circuit-breaker view of the Chalo upstream (see /health). */
export interface UpstreamStatus {
  state: 'closed' | 'open' | 'half_open'
  consecutive_failures: number
  last_success?: string
  last_failure?: string
  retry_after_seconds?: number
}

export interface HealthStatus {
  message: string
  status: string
  browser: boolean
  degraded?: boolean
  upstream?: UpstreamStatus
  latencyMs?: number
}

/** One bus in a multi-bus response: exactly one of tracking/error is set. */
export interface FleetItem {
  bus_code: string
  tracking?: BusTrackingInfo
  error?: ApiErrorBody
}

export interface FleetResponse {
  buses: FleetItem[]
}

/** Payload of a `bus_error` event on the multi-bus stream. */
export interface BusErrorEvent extends ApiErrorBody {
  bus_code: string
}

/** How the UI is currently receiving position updates. */
export type ConnectionMode = 'connecting' | 'live' | 'polling' | 'paused'
