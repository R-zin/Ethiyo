export interface BusPosition {
  latitude: number
  longitude: number
  speed_kmh?: number
  recorded_at?: number
  route_name?: string
  next_stop?: string
  prev_stop?: string
}

export interface BusTrackingInfo {
  bus_code: string
  tracking_url: string
  position?: BusPosition
}

export interface BusRouteInfo {
  bus_code: string
  tracking_url: string
  route_url: string
}

export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: {
    code: string
    message: string
  }
}

export interface HealthStatus {
  message: string
  status: string
  browser: boolean
  latencyMs?: number
}
