import { LocateFixed, MapPinned, ShieldCheck, X } from 'lucide-react'
import { GeoPosition, GeoStatus } from '../../hooks/useGeolocation'
import { estimateEtaMinutes, formatDistance, haversineMeters } from '../../utils/geo'

interface ProximityCardProps {
  bus?: { latitude: number; longitude: number; speed?: number }
  status: GeoStatus
  position: GeoPosition | null
  onStart: () => void
  onStop: () => void
}

/**
 * "How far is my bus?" — opt-in straight-line distance and a naive ETA from
 * the viewer's location. Labelled as approximate because it ignores the road
 * network and stops.
 */
export function ProximityCard({ bus, status, position, onStart, onStop }: ProximityCardProps) {
  if (status === 'unsupported') return null

  const distance =
    bus && position ? haversineMeters(position.latitude, position.longitude, bus.latitude, bus.longitude) : null
  const eta = distance !== null ? estimateEtaMinutes(distance, bus?.speed) : null

  return (
    <section
      aria-label="Distance from you"
      data-testid="proximity-card"
      className="p-4 rounded-lg border border-nordic-200/80 dark:border-nordic-800 bg-white dark:bg-nordic-900 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
    >
      <div className="flex items-start gap-3 min-w-0">
        <div className="w-9 h-9 shrink-0 rounded-md bg-pine-50 dark:bg-pine-950/40 border border-pine-100 dark:border-pine-900 flex items-center justify-center text-pine-600">
          <MapPinned className="w-4 h-4" />
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-nordic-900 dark:text-white">How far is my bus?</h3>
          {status === 'active' && distance !== null ? (
            <p className="text-xs text-nordic-600 dark:text-nordic-300 mt-0.5" data-testid="proximity-result">
              <span className="font-mono font-semibold text-nordic-900 dark:text-white">{formatDistance(distance)}</span>{' '}
              away (straight line)
              {eta !== null ? (
                <>
                  {' · '}≈ <span className="font-mono font-semibold">{eta} min</span> at current speed
                </>
              ) : (
                ' · bus is stationary, no ETA yet'
              )}
            </p>
          ) : status === 'locating' ? (
            <p className="text-xs text-nordic-500 mt-0.5">Locating you…</p>
          ) : status === 'denied' ? (
            <p className="text-xs text-berry-600 mt-0.5">Location permission was denied in your browser.</p>
          ) : status === 'unavailable' ? (
            <p className="text-xs text-berry-600 mt-0.5">Your location is unavailable right now.</p>
          ) : (
            <p className="text-xs text-nordic-500 mt-0.5 flex items-center gap-1">
              <ShieldCheck className="w-3 h-3 text-pine-500" /> Your location stays in your browser.
            </p>
          )}
        </div>
      </div>

      {status === 'active' || status === 'locating' ? (
        <button
          type="button"
          onClick={onStop}
          className="self-start sm:self-auto px-3 py-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 text-xs font-medium text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800 flex items-center gap-1.5"
          aria-label="Stop sharing my location"
        >
          <X className="w-3.5 h-3.5" /> Stop
        </button>
      ) : (
        <button
          type="button"
          onClick={onStart}
          disabled={!bus}
          className="self-start sm:self-auto px-3 py-1.5 rounded-md bg-nordic-900 text-white dark:bg-frost-600 hover:bg-nordic-800 dark:hover:bg-frost-500 text-xs font-medium flex items-center gap-1.5 disabled:opacity-50"
        >
          <LocateFixed className="w-3.5 h-3.5" /> Use my location
        </button>
      )}
    </section>
  )
}
