import { useEffect, useState } from 'react'
import {
  Bus,
  Gauge,
  ArrowRight,
  RefreshCw,
  ExternalLink,
  Clock,
  Radio,
  Compass,
  Share2,
  Check,
  AlertTriangle,
  Pause,
} from 'lucide-react'
import { ApiErrorBody, BusTrackingInfo, ConnectionMode } from '../../types/transit'
import { StatusBadge } from '../common/StatusBadge'
import { MetricTile } from '../common/MetricTile'
import { formatSpeed, formatRelativeTime, formatDateTime } from '../../utils/format'
import { compassPoint, formatDistance } from '../../utils/geo'
import { POLL_INTERVAL_MS } from '../../hooks/useBusTracker'

interface LiveTrackerCardProps {
  data: BusTrackingInfo
  mode: ConnectionMode
  paused: boolean
  onTogglePaused: () => void
  onRefresh: () => void
  isRefreshing?: boolean
  warning?: ApiErrorBody | null
  distanceM?: number
  trailPoints?: number
}

const MODE_LABEL: Record<ConnectionMode, string> = {
  live: 'Live stream',
  polling: `Polling ${POLL_INTERVAL_MS / 1000}s`,
  connecting: 'Connecting…',
  paused: 'Paused',
}

/** Re-renders periodically so relative times ("12s ago") keep counting. */
function useTicker(intervalMs: number) {
  const [, setTick] = useState(0)
  useEffect(() => {
    const id = setInterval(() => setTick(t => t + 1), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
}

function shareUrl(busCode: string): string {
  const url = new URL(window.location.href)
  url.search = ''
  url.searchParams.set('code', busCode)
  return url.toString()
}

export function LiveTrackerCard({
  data,
  mode,
  paused,
  onTogglePaused,
  onRefresh,
  isRefreshing = false,
  warning,
  distanceM = 0,
  trailPoints = 0,
}: LiveTrackerCardProps) {
  useTicker(5000)
  const [shared, setShared] = useState(false)

  const live = data.live
  const hasLiveGps = !!live && Number.isFinite(live.latitude) && Number.isFinite(live.longitude)
  const isStale = !!live?.stale
  const speed = live?.speed ?? 0
  const heading = live?.heading

  const handleShare = async () => {
    const url = shareUrl(data.bus_code)
    try {
      if (typeof navigator.share === 'function') {
        await navigator.share({ title: `Bus ${data.bus_code} · Ethiyo`, url })
        return
      }
      await navigator.clipboard.writeText(url)
      setShared(true)
      setTimeout(() => setShared(false), 2000)
    } catch {
      // Share sheet dismissed or clipboard blocked: nothing to do.
    }
  }

  const signalVariant = !hasLiveGps ? 'idle' : isStale ? 'warning' : 'active'
  const signalLabel = !hasLiveGps ? 'No GPS Fix' : isStale ? 'Stale Signal' : 'Live Telemetry'

  return (
    <div className="bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 rounded-lg p-5 sm:p-6 shadow-sm">
      {/* Header bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-5 border-b border-nordic-100 dark:border-nordic-800/80">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-10 h-10 shrink-0 rounded-md bg-frost-50 dark:bg-frost-950/60 border border-frost-100 dark:border-frost-900 flex items-center justify-center text-frost-600 dark:text-frost-400">
            <Bus className="w-5 h-5" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2.5">
              <h2 className="text-xl font-bold tracking-tight text-nordic-900 dark:text-white font-mono">
                {data.bus_code}
              </h2>
              <StatusBadge variant={signalVariant} pulse={hasLiveGps && !isStale}>
                {signalLabel}
              </StatusBadge>
            </div>
            <p className="text-xs text-nordic-500 dark:text-nordic-400 mt-0.5 truncate">
              {live?.route_name || 'Standard Transit Service'}
              {(live?.operator || live?.vehicle_code) && (
                <span className="font-mono">
                  {' · '}
                  {[live?.operator, live?.vehicle_code].filter(Boolean).join(' · ')}
                </span>
              )}
            </p>
          </div>
        </div>

        {/* Live-update & action controls */}
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={onTogglePaused}
            data-testid="connection-mode"
            data-mode={mode}
            className={`px-2.5 py-1.5 rounded-md text-xs font-medium border flex items-center gap-1.5 transition-colors ${
              paused
                ? 'bg-nordic-50 text-nordic-600 border-nordic-200 dark:bg-nordic-800/50 dark:text-nordic-400 dark:border-nordic-700'
                : 'bg-frost-50 text-frost-700 border-frost-200 dark:bg-frost-950/50 dark:text-frost-300 dark:border-frost-800'
            }`}
            aria-label={paused ? 'Resume live updates' : 'Pause live updates'}
            aria-pressed={!paused}
            title={mode === 'live' ? 'Receiving server-pushed updates' : undefined}
          >
            {paused ? (
              <Pause className="w-3.5 h-3.5" />
            ) : (
              <Radio className={`w-3.5 h-3.5 ${mode === 'live' ? 'animate-pulse text-frost-500' : ''}`} />
            )}
            <span>{MODE_LABEL[mode]}</span>
          </button>

          <button
            type="button"
            onClick={onRefresh}
            disabled={isRefreshing}
            className="p-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800 transition-colors disabled:opacity-50"
            title="Refresh telemetry now"
            aria-label="Refresh telemetry now"
          >
            <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-frost-500' : ''}`} />
          </button>

          <button
            type="button"
            onClick={handleShare}
            className="p-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800 transition-colors"
            title="Share a link to this bus"
            aria-label={shared ? 'Link copied' : 'Share link to this bus'}
          >
            {shared ? <Check className="w-4 h-4 text-pine-500" /> : <Share2 className="w-4 h-4" />}
          </button>

          {data.tracking_url && (
            <a
              href={data.tracking_url}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 text-nordic-700 dark:text-nordic-200 hover:bg-nordic-50 dark:hover:bg-nordic-800 transition-colors"
              title="Open raw tracking URL"
              aria-label="Open live tracking endpoint"
            >
              <ExternalLink className="w-4 h-4" />
            </a>
          )}
        </div>
      </div>

      {warning && (
        <div
          role="status"
          className="mt-4 px-3 py-2 rounded-md border border-amber-200 dark:border-amber-900/60 bg-amber-50 dark:bg-amber-950/30 text-xs text-amber-700 dark:text-amber-400 flex items-start gap-2"
        >
          <AlertTriangle className="w-3.5 h-3.5 mt-0.5 shrink-0" />
          <span>
            Live feed issue ({warning.code}): {warning.message}. Showing the last known position.
          </span>
        </div>
      )}

      {/* Metric Tiles */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mt-5">
        <MetricTile
          label="Current Speed"
          value={formatSpeed(speed)}
          icon={<Gauge className="w-4 h-4" />}
          variant={speed > 0 ? 'highlight' : 'default'}
          subtext={speed > 0 ? 'In Transit' : 'Stationary / Idle'}
        />

        <MetricTile
          label="Next Stop"
          value={live?.next_stop || '—'}
          icon={<ArrowRight className="w-4 h-4 text-frost-500" />}
          subtext={live?.previous_stop ? `From: ${live.previous_stop}` : 'Pending route sync'}
        />

        <MetricTile
          label="Last Reported"
          value={formatRelativeTime(live?.recorded_at)}
          icon={<Clock className="w-4 h-4" />}
          subtext={isStale ? `Stale · ${formatDateTime(live?.recorded_at)}` : formatDateTime(live?.recorded_at)}
        />

        <MetricTile
          label="Heading"
          value={typeof heading === 'number' ? compassPoint(heading) : '—'}
          icon={
            <Compass
              className="w-4 h-4 text-pine-500 transition-transform"
              style={typeof heading === 'number' ? { transform: `rotate(${heading - 45}deg)` } : undefined}
            />
          }
          subtext={
            typeof heading === 'number'
              ? `${Math.round(heading)}° · ${formatDistance(distanceM)} tracked`
              : trailPoints > 1
              ? `${formatDistance(distanceM)} tracked`
              : 'Awaiting movement'
          }
        />
      </div>

      {/* Stop Progression Ribbon */}
      {(live?.previous_stop || live?.next_stop) && (
        <div className="mt-5 p-3.5 rounded-md bg-nordic-50 dark:bg-nordic-950/60 border border-nordic-100 dark:border-nordic-800/80 flex items-center justify-between text-xs font-mono min-w-0">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-nordic-400 hidden sm:inline">Previous:</span>
            <span className="font-semibold text-nordic-800 dark:text-nordic-200 truncate">
              {live.previous_stop || 'Departure Terminal'}
            </span>
          </div>

          <ArrowRight className="w-4 h-4 text-frost-500 flex-shrink-0 mx-2" />

          <div className="flex items-center gap-2 min-w-0">
            <span className="text-nordic-400 hidden sm:inline">Approaching:</span>
            <span className="font-semibold text-frost-600 dark:text-frost-400 truncate">
              {live.next_stop || 'Terminal Station'}
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
