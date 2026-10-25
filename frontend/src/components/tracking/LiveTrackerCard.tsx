import { useState, useEffect } from 'react'
import { Bus, Gauge, ArrowRight, RefreshCw, ExternalLink, Clock, Radio } from 'lucide-react'
import { BusTrackingInfo } from '../../types/transit'
import { StatusBadge } from '../common/StatusBadge'
import { MetricTile } from '../common/MetricTile'
import { formatSpeed, formatRelativeTime, formatDateTime } from '../../utils/format'

interface LiveTrackerCardProps {
  data: BusTrackingInfo
  onRefresh: () => void
  isRefreshing?: boolean
  lastUpdated?: number
}

const POLL_INTERVAL_SEC = 15

export function LiveTrackerCard({
  data,
  onRefresh,
  isRefreshing = false,
  lastUpdated,
}: LiveTrackerCardProps) {
  const [autoPoll, setAutoPoll] = useState(true)
  const [countdown, setCountdown] = useState(POLL_INTERVAL_SEC)

  const pos = data.position
  const hasLiveGps = !!pos && typeof pos.latitude === 'number'

  // Countdown timer for auto-refresh
  useEffect(() => {
    if (!autoPoll) return

    const timer = setInterval(() => {
      setCountdown(prev => {
        if (prev <= 1) {
          onRefresh()
          return POLL_INTERVAL_SEC
        }
        return prev - 1
      })
    }, 1000)

    return () => clearInterval(timer)
  }, [autoPoll, onRefresh])

  // Reset countdown when refresh occurs
  useEffect(() => {
    setCountdown(POLL_INTERVAL_SEC)
  }, [lastUpdated])

  return (
    <div className="bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 rounded-lg p-5 sm:p-6 shadow-sm">
      {/* Header bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-5 border-b border-nordic-100 dark:border-nordic-800/80">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-md bg-frost-50 dark:bg-frost-950/60 border border-frost-100 dark:border-frost-900 flex items-center justify-center text-frost-600 dark:text-frost-400">
            <Bus className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2.5">
              <h2 className="text-xl font-bold tracking-tight text-nordic-900 dark:text-white font-mono">
                {data.bus_code}
              </h2>
              <StatusBadge
                variant={hasLiveGps ? 'active' : 'idle'}
                pulse={hasLiveGps}
              >
                {hasLiveGps ? 'Live Telemetry' : 'Static Schedule'}
              </StatusBadge>
            </div>
            <p className="text-xs text-nordic-500 dark:text-nordic-400 mt-0.5">
              {pos?.route_name || 'Standard Transit Service'}
            </p>
          </div>
        </div>

        {/* Polling & Refresh Controls */}
        <div className="flex items-center gap-2">
          {/* Auto Refresh pill */}
          <button
            type="button"
            onClick={() => setAutoPoll(!autoPoll)}
            className={`px-2.5 py-1.5 rounded-md text-xs font-medium border flex items-center gap-1.5 transition-colors ${
              autoPoll
                ? 'bg-frost-50 text-frost-700 border-frost-200 dark:bg-frost-950/50 dark:text-frost-300 dark:border-frost-800'
                : 'bg-nordic-50 text-nordic-600 border-nordic-200 dark:bg-nordic-800/50 dark:text-nordic-400 dark:border-nordic-700'
            }`}
            aria-label="Toggle auto refresh polling"
          >
            <Radio className={`w-3.5 h-3.5 ${autoPoll ? 'animate-pulse text-frost-500' : ''}`} />
            <span>Auto: {autoPoll ? `${countdown}s` : 'Off'}</span>
          </button>

          {/* Manual Refresh */}
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

          {/* External Chalo link */}
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

      {/* Metric Tiles */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mt-5">
        <MetricTile
          label="Current Speed"
          value={formatSpeed(pos?.speed_kmh)}
          icon={<Gauge className="w-4 h-4" />}
          variant={pos?.speed_kmh && pos.speed_kmh > 0 ? 'highlight' : 'default'}
          subtext={pos?.speed_kmh && pos.speed_kmh > 0 ? 'In Transit' : 'Stationary / Idle'}
        />

        <MetricTile
          label="Next Stop"
          value={pos?.next_stop || '—'}
          icon={<ArrowRight className="w-4 h-4 text-frost-500" />}
          subtext={pos?.prev_stop ? `From: ${pos.prev_stop}` : 'Pending route sync'}
        />

        <MetricTile
          label="Last Reported"
          value={formatRelativeTime(pos?.recorded_at || lastUpdated)}
          icon={<Clock className="w-4 h-4" />}
          subtext={formatDateTime(pos?.recorded_at || lastUpdated)}
        />

        <MetricTile
          label="GPS Signal"
          value={hasLiveGps ? 'Locked' : 'Unavailable'}
          icon={<Radio className="w-4 h-4 text-pine-500" />}
          subtext={hasLiveGps ? 'Telemetry Streaming' : 'Checking scheduler'}
        />
      </div>

      {/* Stop Progression Ribbon */}
      {(pos?.prev_stop || pos?.next_stop) && (
        <div className="mt-5 p-3.5 rounded-md bg-nordic-50 dark:bg-nordic-950/60 border border-nordic-100 dark:border-nordic-800/80 flex items-center justify-between text-xs font-mono">
          <div className="flex items-center gap-2 truncate">
            <span className="text-nordic-400">Previous:</span>
            <span className="font-semibold text-nordic-800 dark:text-nordic-200 truncate">
              {pos.prev_stop || 'Departure Terminal'}
            </span>
          </div>

          <ArrowRight className="w-4 h-4 text-frost-500 flex-shrink-0 mx-2" />

          <div className="flex items-center gap-2 truncate">
            <span className="text-nordic-400">Approaching:</span>
            <span className="font-semibold text-frost-600 dark:text-frost-400 truncate">
              {pos.next_stop || 'Terminal Station'}
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
