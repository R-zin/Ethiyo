import { useCallback, useEffect, useRef, useState } from 'react'

export type GeoStatus = 'idle' | 'locating' | 'active' | 'denied' | 'unavailable' | 'unsupported'

export interface GeoPosition {
  latitude: number
  longitude: number
  accuracy: number
}

/**
 * Opt-in device location. Nothing is requested until `start()` is called
 * from a user gesture, and the position never leaves the browser: distance
 * and ETA are computed client-side.
 */
export function useGeolocation() {
  const supported = typeof navigator !== 'undefined' && 'geolocation' in navigator
  const [status, setStatus] = useState<GeoStatus>(supported ? 'idle' : 'unsupported')
  const [position, setPosition] = useState<GeoPosition | null>(null)
  const watchId = useRef<number | null>(null)

  const stop = useCallback(() => {
    if (watchId.current !== null && supported) {
      navigator.geolocation.clearWatch(watchId.current)
    }
    watchId.current = null
    setPosition(null)
    setStatus(supported ? 'idle' : 'unsupported')
  }, [supported])

  const start = useCallback(() => {
    if (!supported || watchId.current !== null) return
    setStatus('locating')
    watchId.current = navigator.geolocation.watchPosition(
      pos => {
        setPosition({
          latitude: pos.coords.latitude,
          longitude: pos.coords.longitude,
          accuracy: pos.coords.accuracy,
        })
        setStatus('active')
      },
      err => {
        if (watchId.current !== null) navigator.geolocation.clearWatch(watchId.current)
        watchId.current = null
        setStatus(err.code === err.PERMISSION_DENIED ? 'denied' : 'unavailable')
      },
      { enableHighAccuracy: true, maximumAge: 15_000, timeout: 20_000 }
    )
  }, [supported])

  useEffect(() => {
    return () => {
      if (watchId.current !== null && supported) navigator.geolocation.clearWatch(watchId.current)
    }
  }, [supported])

  return { status, position, start, stop }
}
