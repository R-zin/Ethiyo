import { Bell, BellRing, X } from 'lucide-react'
import { AlertConfig } from '../../utils/alerts'
import { ActiveAlert, NotifyPermission } from '../../hooks/useAlerts'
import { GeoStatus } from '../../hooks/useGeolocation'

export const APPROACH_CHOICES = [2, 5, 10, 15] as const

interface AlertsCardProps {
  busCode: string
  config: AlertConfig
  permission: NotifyPermission
  locationStatus: GeoStatus
  onApproachChange: (minutes: number | null) => void
  onOfflineChange: (enabled: boolean) => void
  onEnableLocation: () => void
}

const PERMISSION_TEXT: Record<NotifyPermission, string> = {
  granted: 'Browser notifications are on.',
  denied: 'Browser notifications are blocked, so alerts will only appear on this page.',
  default: 'Your browser will ask to allow notifications.',
  unsupported: "This browser can't show notifications, so alerts will appear on this page.",
}

/** Lets the viewer arm "almost here" and "stopped reporting" alerts. */
export function AlertsCard({
  busCode,
  config,
  permission,
  locationStatus,
  onApproachChange,
  onOfflineChange,
  onEnableLocation,
}: AlertsCardProps) {
  const needsLocation =
    config.approachMinutes !== null && locationStatus !== 'active' && locationStatus !== 'locating'
  const anyArmed = config.approachMinutes !== null || config.offline

  const handleApproach = (value: string) => {
    const minutes = value === 'off' ? null : Number(value)
    onApproachChange(minutes)
    // Selecting a threshold is a user gesture: a good moment to ask for location.
    if (minutes !== null && locationStatus === 'idle') onEnableLocation()
  }

  return (
    <section
      aria-label={`Alerts for bus ${busCode}`}
      data-testid="alerts-card"
      className="p-4 rounded-lg border border-nordic-200/80 dark:border-nordic-800 bg-white dark:bg-nordic-900 space-y-3"
    >
      <div className="flex items-center gap-2">
        {anyArmed ? <BellRing className="w-4 h-4 text-frost-500" /> : <Bell className="w-4 h-4 text-nordic-400" />}
        <h3 className="text-sm font-semibold text-nordic-900 dark:text-white">Alerts</h3>
        <span className="text-xs text-nordic-400">· work while this page is open</span>
      </div>

      <div className="flex flex-col sm:flex-row sm:items-center gap-x-6 gap-y-2.5 text-xs text-nordic-700 dark:text-nordic-300">
        <label htmlFor="approach-alert" className="flex items-center gap-2">
          <span>Notify me when it's</span>
          <select
            id="approach-alert"
            value={config.approachMinutes === null ? 'off' : String(config.approachMinutes)}
            onChange={e => handleApproach(e.target.value)}
            className="px-2 py-1 rounded-md border border-nordic-200 dark:border-nordic-700 bg-white dark:bg-nordic-800 text-nordic-900 dark:text-nordic-100"
          >
            <option value="off">off</option>
            {APPROACH_CHOICES.map(m => (
              <option key={m} value={m}>
                ≤ {m} min
              </option>
            ))}
          </select>
          <span>away</span>
        </label>

        <label htmlFor="offline-alert" className="flex items-center gap-2 cursor-pointer">
          <input
            id="offline-alert"
            type="checkbox"
            checked={config.offline}
            onChange={e => onOfflineChange(e.target.checked)}
            className="rounded border-nordic-300 text-frost-600 focus:ring-frost-500"
          />
          <span>Notify me if it stops reporting</span>
        </label>
      </div>

      {needsLocation && (
        <p className="text-xs text-amber-700 dark:text-amber-400" data-testid="alerts-need-location">
          The arrival alert needs your location.{' '}
          <button type="button" onClick={onEnableLocation} className="underline font-medium">
            Use my location
          </button>
        </p>
      )}
      {anyArmed && <p className="text-[11px] text-nordic-500">{PERMISSION_TEXT[permission]}</p>}
    </section>
  )
}

interface AlertBannersProps {
  alerts: ActiveAlert[]
  onDismiss: (id: number) => void
}

/** Fired alerts, shown in the page until dismissed. */
export function AlertBanners({ alerts, onDismiss }: AlertBannersProps) {
  if (alerts.length === 0) return null
  return (
    <div className="space-y-2">
      {alerts.map(a => (
        <div
          key={a.id}
          role="alert"
          data-testid="alert-banner"
          data-kind={a.kind}
          className={`flex items-start gap-3 p-3.5 rounded-lg border shadow-sm ${
            a.kind === 'approach'
              ? 'border-pine-200 bg-pine-50 text-pine-800 dark:border-pine-900 dark:bg-pine-950/40 dark:text-pine-300'
              : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300'
          }`}
        >
          <BellRing className="w-4 h-4 mt-0.5 shrink-0" />
          <div className="text-sm flex-1 min-w-0">
            <p className="font-semibold">{a.title}</p>
            <p className="text-xs mt-0.5 opacity-90">{a.body}</p>
          </div>
          <button
            type="button"
            onClick={() => onDismiss(a.id)}
            className="p-1 rounded hover:bg-black/5 dark:hover:bg-white/10"
            aria-label="Dismiss alert"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      ))}
    </div>
  )
}
