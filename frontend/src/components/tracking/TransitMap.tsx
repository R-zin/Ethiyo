import { useEffect, useRef, useState } from 'react'
import L from 'leaflet'
import { Navigation, MapPin, LocateFixed } from 'lucide-react'
import { formatCoordinates } from '../../utils/format'
import { PositionFix } from '../../types/transit'
import { buildPopupContent, busIconHtml } from './mapMarkup'

interface TransitMapProps {
  latitude?: number
  longitude?: number
  busCode: string
  routeName?: string
  speedKmh?: number
  heading?: number
  stale?: boolean
  trail?: PositionFix[]
  userPosition?: { latitude: number; longitude: number } | null
}

const DEFAULT_CENTER: L.LatLngTuple = [12.9716, 77.5946]

export function TransitMap({
  latitude,
  longitude,
  busCode,
  routeName,
  speedKmh,
  heading,
  stale = false,
  trail = [],
  userPosition = null,
}: TransitMapProps) {
  const mapContainerRef = useRef<HTMLDivElement>(null)
  const mapRef = useRef<L.Map | null>(null)
  const markerRef = useRef<L.Marker | null>(null)
  const trailRef = useRef<L.Polyline | null>(null)
  const userRef = useRef<L.CircleMarker | null>(null)
  const centeredRef = useRef(false)
  // Follow mode keeps the bus in view; dragging the map hands control to the user.
  const [following, setFollowing] = useState(true)
  const followingRef = useRef(following)
  followingRef.current = following

  const hasCoords =
    typeof latitude === 'number' && typeof longitude === 'number' && !isNaN(latitude) && !isNaN(longitude)

  // Create the map once and destroy it on unmount (e.g. switching tabs);
  // otherwise every remount leaks a Leaflet instance and its listeners.
  useEffect(() => {
    if (!mapContainerRef.current) return
    const map = L.map(mapContainerRef.current, { zoomControl: false, attributionControl: true }).setView(
      DEFAULT_CENTER,
      12
    )
    L.control.zoom({ position: 'bottomright' }).addTo(map)
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors',
    }).addTo(map)
    map.on('dragstart', () => setFollowing(false))
    mapRef.current = map

    return () => {
      map.remove()
      mapRef.current = null
      markerRef.current = null
      trailRef.current = null
      userRef.current = null
      centeredRef.current = false
    }
  }, [])

  // Bus marker, heading arrow and popup.
  useEffect(() => {
    const map = mapRef.current
    if (!map || !hasCoords) return

    const icon = L.divIcon({
      className: 'custom-bus-marker',
      html: busIconHtml(heading, stale),
      iconSize: [40, 40],
      iconAnchor: [20, 20],
      popupAnchor: [0, -20],
    })
    const latLng: L.LatLngTuple = [latitude, longitude]
    if (markerRef.current) {
      markerRef.current.setLatLng(latLng).setIcon(icon)
    } else {
      markerRef.current = L.marker(latLng, { icon, title: `Bus ${busCode}` }).addTo(map)
    }
    markerRef.current.bindPopup(buildPopupContent({ busCode, routeName, speedKmh, latitude, longitude, stale }))

    if (!centeredRef.current) {
      map.setView(latLng, 15)
      centeredRef.current = true
    } else if (followingRef.current) {
      map.panTo(latLng, { animate: true, duration: 0.8 })
    }
  }, [latitude, longitude, hasCoords, busCode, routeName, speedKmh, heading, stale])

  // Breadcrumb trail of recent positions.
  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    const points = trail.map(p => [p.latitude, p.longitude] as L.LatLngTuple)
    if (points.length < 2) {
      trailRef.current?.remove()
      trailRef.current = null
      return
    }
    if (trailRef.current) {
      trailRef.current.setLatLngs(points)
    } else {
      trailRef.current = L.polyline(points, {
        color: '#0ea5e9',
        weight: 4,
        opacity: 0.65,
        lineCap: 'round',
        className: 'bus-trail',
      }).addTo(map)
    }
  }, [trail])

  // The viewer's own location, when they opted in.
  useEffect(() => {
    const map = mapRef.current
    if (!map) return
    if (!userPosition) {
      userRef.current?.remove()
      userRef.current = null
      return
    }
    const latLng: L.LatLngTuple = [userPosition.latitude, userPosition.longitude]
    if (userRef.current) {
      userRef.current.setLatLng(latLng)
    } else {
      userRef.current = L.circleMarker(latLng, {
        radius: 7,
        color: '#ffffff',
        weight: 2,
        fillColor: '#10b981',
        fillOpacity: 1,
        className: 'user-location',
      })
        .bindTooltip('You are here')
        .addTo(map)
    }
  }, [userPosition])

  const handleCenterOnBus = () => {
    if (mapRef.current && hasCoords) {
      setFollowing(true)
      mapRef.current.flyTo([latitude, longitude], 16, { duration: 1 })
      markerRef.current?.openPopup()
    }
  }

  return (
    <div className="relative w-full h-[360px] sm:h-[420px] rounded-lg border border-nordic-200/80 dark:border-nordic-800 overflow-hidden bg-nordic-100 dark:bg-nordic-900">
      <div ref={mapContainerRef} className="w-full h-full z-10" data-testid="transit-map" />

      {/* Floating HUD Badges */}
      <div className="absolute top-3 left-3 z-20 flex flex-col gap-2 pointer-events-none max-w-[calc(100%-4.5rem)]">
        {hasCoords ? (
          <div className="px-3 py-1.5 rounded-md bg-white/95 dark:bg-nordic-900/95 border border-nordic-200/80 dark:border-nordic-800 shadow-sm backdrop-blur text-xs font-mono text-nordic-700 dark:text-nordic-300 flex items-center gap-2">
            <span className={`w-2 h-2 rounded-full ${stale ? 'bg-amber-500' : 'bg-pine-500 animate-pulse'}`} />
            <span className="truncate">{formatCoordinates(latitude, longitude)}</span>
          </div>
        ) : (
          <div className="px-3 py-1.5 rounded-md bg-white/95 dark:bg-nordic-900/95 border border-amber-200 dark:border-amber-900/60 shadow-sm backdrop-blur text-xs font-medium text-amber-700 dark:text-amber-400 flex items-center gap-1.5">
            <MapPin className="w-3.5 h-3.5" />
            <span>GPS not currently broadcasting coordinates</span>
          </div>
        )}
        {trail.length > 1 && (
          <div className="px-2.5 py-1 rounded-md bg-white/95 dark:bg-nordic-900/95 border border-nordic-200/80 dark:border-nordic-800 shadow-sm text-[11px] text-nordic-600 dark:text-nordic-300 w-fit">
            Trail: last {trail.length} positions
          </div>
        )}
      </div>

      {hasCoords && (
        <button
          type="button"
          onClick={handleCenterOnBus}
          className={`absolute top-3 right-3 z-20 p-2.5 rounded-md border shadow-sm transition-colors ${
            following
              ? 'bg-white dark:bg-nordic-900 border-nordic-200/80 dark:border-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800'
              : 'bg-frost-600 border-frost-600 text-white hover:bg-frost-500'
          }`}
          title={following ? 'Following the bus' : 'Re-center and follow the bus'}
          aria-label={following ? 'Center map on vehicle location' : 'Re-center map and follow the bus'}
          aria-pressed={following}
        >
          {following ? (
            <Navigation className="w-4 h-4 text-frost-600 dark:text-frost-400" />
          ) : (
            <LocateFixed className="w-4 h-4" />
          )}
        </button>
      )}
    </div>
  )
}
