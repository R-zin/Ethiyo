import { useEffect, useRef } from 'react'
import L from 'leaflet'
import { Navigation, MapPin } from 'lucide-react'
import { formatCoordinates } from '../../utils/format'

interface TransitMapProps {
  latitude?: number
  longitude?: number
  busCode: string
  routeName?: string
  speedKmh?: number
}

export function TransitMap({
  latitude,
  longitude,
  busCode,
  routeName,
  speedKmh,
}: TransitMapProps) {
  const mapContainerRef = useRef<HTMLDivElement>(null)
  const mapInstanceRef = useRef<L.Map | null>(null)
  const markerRef = useRef<L.Marker | null>(null)

  const hasCoords =
    typeof latitude === 'number' &&
    typeof longitude === 'number' &&
    !isNaN(latitude) &&
    !isNaN(longitude)

  useEffect(() => {
    if (!mapContainerRef.current) return

    // Default to Bangalore/Delhi or coordinate center if present
    const centerLat = hasCoords ? latitude : 12.9716
    const centerLng = hasCoords ? longitude : 77.5946

    if (!mapInstanceRef.current) {
      const map = L.map(mapContainerRef.current, {
        zoomControl: false,
        attributionControl: true,
      }).setView([centerLat, centerLng], hasCoords ? 15 : 12)

      L.control
        .zoom({
          position: 'bottomright',
        })
        .addTo(map)

      // OpenStreetMap standard tiles
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        attribution: '&copy; OpenStreetMap contributors',
      }).addTo(map)

      mapInstanceRef.current = map
    }

    const map = mapInstanceRef.current

    if (hasCoords) {
      const customIcon = L.divIcon({
        className: 'custom-bus-marker',
        html: `
          <div class="relative flex items-center justify-center w-8 h-8">
            <span class="animate-ping absolute inline-flex h-8 w-8 rounded-full bg-frost-400 opacity-60"></span>
            <div class="relative flex items-center justify-center w-8 h-8 rounded-full bg-nordic-900 border-2 border-white shadow-md text-white font-bold text-xs">
              🚌
            </div>
          </div>
        `,
        iconSize: [32, 32],
        iconAnchor: [16, 16],
        popupAnchor: [0, -18],
      })

      if (markerRef.current) {
        markerRef.current.setLatLng([latitude, longitude])
      } else {
        const marker = L.marker([latitude, longitude], { icon: customIcon }).addTo(map)
        markerRef.current = marker
      }

      const popupContent = `
        <div style="font-family: inherit;">
          <div style="font-weight: 600; font-size: 13px; color: #0f172a; margin-bottom: 2px;">
            Bus ${busCode}
          </div>
          <div style="font-size: 11px; color: #64748b; margin-bottom: 6px;">
            ${routeName || 'Live Transit Route'}
          </div>
          <div style="display: flex; gap: 8px; font-size: 11px; font-family: monospace;">
            <span style="color: #0284c7; font-weight: 600;">${speedKmh ? Math.round(speedKmh) : 0} km/h</span>
            <span style="color: #94a3b8;">·</span>
            <span style="color: #475569;">${latitude.toFixed(4)}, ${longitude.toFixed(4)}</span>
          </div>
        </div>
      `
      markerRef.current.bindPopup(popupContent)

      map.panTo([latitude, longitude], { animate: true, duration: 0.8 })
    }

    return () => {
      // Keep map initialized between prop updates
    }
  }, [latitude, longitude, hasCoords, busCode, routeName, speedKmh])

  const handleCenterOnBus = () => {
    if (mapInstanceRef.current && hasCoords) {
      mapInstanceRef.current.flyTo([latitude, longitude], 16, { duration: 1 })
      markerRef.current?.openPopup()
    }
  }

  return (
    <div className="relative w-full h-[360px] sm:h-[420px] rounded-lg border border-nordic-200/80 dark:border-nordic-800 overflow-hidden bg-nordic-100 dark:bg-nordic-900">
      <div ref={mapContainerRef} className="w-full h-full z-10" />

      {/* Floating HUD Badges */}
      <div className="absolute top-3 left-3 z-20 flex flex-col gap-2 pointer-events-none">
        {hasCoords ? (
          <div className="px-3 py-1.5 rounded-md bg-white/95 dark:bg-nordic-900/95 border border-nordic-200/80 dark:border-nordic-800 shadow-sm backdrop-blur text-xs font-mono text-nordic-700 dark:text-nordic-300 flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-pine-500 animate-pulse" />
            <span>{formatCoordinates(latitude, longitude)}</span>
          </div>
        ) : (
          <div className="px-3 py-1.5 rounded-md bg-white/95 dark:bg-nordic-900/95 border border-amber-200 dark:border-amber-900/60 shadow-sm backdrop-blur text-xs font-medium text-amber-700 dark:text-amber-400 flex items-center gap-1.5">
            <MapPin className="w-3.5 h-3.5" />
            <span>GPS not currently broadcasting coordinates</span>
          </div>
        )}
      </div>

      {hasCoords && (
        <button
          onClick={handleCenterOnBus}
          className="absolute top-3 right-3 z-20 p-2.5 rounded-md bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800 shadow-sm transition-colors"
          title="Recenter map on bus position"
          aria-label="Center map on vehicle location"
        >
          <Navigation className="w-4 h-4 text-frost-600 dark:text-frost-400" />
        </button>
      )}
    </div>
  )
}
