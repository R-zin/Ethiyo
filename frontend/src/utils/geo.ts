const EARTH_RADIUS_M = 6371008.8

const toRad = (deg: number) => (deg * Math.PI) / 180

/** Great-circle distance in meters. */
export function haversineMeters(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const dLat = toRad(lat2 - lat1)
  const dLon = toRad(lon2 - lon1)
  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(a)))
}

const COMPASS = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'] as const

/** Converts a bearing in degrees to an 8-point compass label. */
export function compassPoint(headingDeg: number): (typeof COMPASS)[number] {
  const normalized = ((headingDeg % 360) + 360) % 360
  return COMPASS[Math.round(normalized / 45) % 8]
}

export function formatDistance(meters: number): string {
  if (!Number.isFinite(meters) || meters < 0) return '—'
  if (meters < 1000) return `${Math.round(meters)} m`
  return `${(meters / 1000).toFixed(meters < 10_000 ? 1 : 0)} km`
}

/** Below this speed the bus is treated as stopped and no ETA is estimated. */
export const MIN_ETA_SPEED_KMH = 3

/**
 * Naive straight-line arrival estimate: distance ÷ current speed. It ignores
 * the road network and stops, so callers must label it as approximate.
 * Returns null when the bus is (nearly) stationary.
 */
export function estimateEtaMinutes(distanceMeters: number, speedKmh?: number): number | null {
  if (!speedKmh || speedKmh < MIN_ETA_SPEED_KMH || !Number.isFinite(distanceMeters)) return null
  return Math.max(1, Math.round(distanceMeters / ((speedKmh * 1000) / 60)))
}
