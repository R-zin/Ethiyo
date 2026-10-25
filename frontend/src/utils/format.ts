export function formatRelativeTime(timestampMs?: number): string {
  if (!timestampMs) return 'Live now'
  
  const diffSec = Math.floor((Date.now() - timestampMs) / 1000)
  if (diffSec < 5) return 'Just now'
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHour = Math.floor(diffMin / 60)
  return `${diffHour}h ago`
}

export function formatCoordinates(lat: number, lng: number): string {
  const latDir = lat >= 0 ? 'N' : 'S'
  const lngDir = lng >= 0 ? 'E' : 'W'
  return `${Math.abs(lat).toFixed(4)}° ${latDir}, ${Math.abs(lng).toFixed(4)}° ${lngDir}`
}

export function formatSpeed(speedKmh?: number): string {
  if (speedKmh === undefined || speedKmh === null || isNaN(speedKmh)) {
    return '0 km/h'
  }
  return `${Math.round(speedKmh)} km/h`
}

export function formatDateTime(timestampMs?: number): string {
  if (!timestampMs) return '—'
  return new Date(timestampMs).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
