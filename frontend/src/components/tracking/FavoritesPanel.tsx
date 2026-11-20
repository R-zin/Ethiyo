import { Star, X, Radio } from 'lucide-react'
import { ConnectionMode } from '../../types/transit'
import { FleetEntry } from '../../hooks/useFleet'
import { StatusVariant } from '../common/StatusBadge'
import { formatRelativeTime, formatSpeed } from '../../utils/format'

interface FavoritesPanelProps {
  favorites: string[]
  entries: Record<string, FleetEntry>
  mode: ConnectionMode
  activeCode?: string
  onSelect: (code: string) => void
  onRemove: (code: string) => void
}

export interface FleetStatus {
  label: string
  variant: StatusVariant
}

/** How trustworthy a saved bus's latest status is, in one short label. */
export function fleetStatus(entry?: FleetEntry): FleetStatus {
  if (!entry) return { label: 'Loading…', variant: 'neutral' }
  if (entry.error) {
    return entry.error.code === 'BUS_NOT_FOUND'
      ? { label: 'Not live', variant: 'idle' }
      : { label: 'Unavailable', variant: 'error' }
  }
  const t = entry.tracking
  if (!t?.live) return { label: 'No GPS fix', variant: 'idle' }
  if (t.degraded) return { label: 'Last known', variant: 'warning' }
  if (t.live.stale) return { label: 'Stale', variant: 'warning' }
  return { label: 'Live', variant: 'active' }
}

const DOT: Record<StatusVariant, string> = {
  active: 'bg-pine-500',
  idle: 'bg-nordic-400',
  warning: 'bg-amber-500',
  error: 'bg-berry-500',
  neutral: 'bg-nordic-300 animate-pulse',
}

const MODE_LABEL: Record<ConnectionMode, string> = {
  live: 'Live',
  polling: 'Polling',
  connecting: 'Connecting…',
  paused: 'Paused',
}

export function FavoritesPanel({ favorites, entries, mode, activeCode, onSelect, onRemove }: FavoritesPanelProps) {
  if (favorites.length === 0) return null

  return (
    <section aria-label="My buses" data-testid="favorites-panel" className="space-y-2.5">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-nordic-900 dark:text-white flex items-center gap-1.5">
          <Star className="w-4 h-4 text-amber-500 fill-amber-400" /> My buses
          <span className="text-xs font-normal text-nordic-400">({favorites.length})</span>
        </h2>
        <span
          data-testid="fleet-mode"
          data-mode={mode}
          className="text-[11px] font-medium text-nordic-500 dark:text-nordic-400 flex items-center gap-1"
        >
          <Radio className={`w-3 h-3 ${mode === 'live' ? 'text-frost-500 animate-pulse' : ''}`} />
          {MODE_LABEL[mode]}
        </span>
      </div>

      <ul className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5">
        {favorites.map(code => {
          const entry = entries[code]
          const status = fleetStatus(entry)
          const live = entry?.tracking?.live
          const isActive = code === activeCode
          return (
            <li
              key={code}
              data-testid="favorite-card"
              data-code={code}
              data-status={status.label}
              className={`relative rounded-lg border bg-white dark:bg-nordic-900 transition-colors ${
                isActive
                  ? 'border-frost-400 dark:border-frost-600 ring-1 ring-frost-400/40'
                  : 'border-nordic-200/80 dark:border-nordic-800 hover:border-frost-300 dark:hover:border-frost-700'
              }`}
            >
              <button
                type="button"
                onClick={() => onSelect(code)}
                className="w-full text-left p-3 pr-9 min-w-0"
                aria-label={`Track bus ${code} (${status.label})`}
                aria-current={isActive ? 'true' : undefined}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className="font-mono font-semibold text-sm text-nordic-900 dark:text-white">{code}</span>
                  <span className="flex items-center gap-1 text-[11px] text-nordic-500 dark:text-nordic-400 shrink-0">
                    <span className={`w-1.5 h-1.5 rounded-full ${DOT[status.variant]}`} />
                    {status.label}
                  </span>
                </div>
                <p className="mt-1 text-xs text-nordic-500 dark:text-nordic-400 truncate">
                  {entry?.error
                    ? entry.error.message
                    : live
                    ? `${formatSpeed(live.speed)} · ${live.next_stop ? `next ${live.next_stop}` : live.route_name || 'en route'}`
                    : '—'}
                </p>
                {live && (
                  <p className="mt-0.5 text-[11px] font-mono text-nordic-400">{formatRelativeTime(live.recorded_at)}</p>
                )}
              </button>
              <button
                type="button"
                onClick={() => onRemove(code)}
                className="absolute top-2 right-2 p-1 rounded text-nordic-400 hover:text-berry-600 hover:bg-nordic-50 dark:hover:bg-nordic-800"
                aria-label={`Remove ${code} from My buses`}
                title="Remove from My buses"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
