import { useState } from 'react'
import { Copy, Check, ExternalLink, Globe, Layers, AlertCircle } from 'lucide-react'
import { ApiErrorBody, BusRouteInfo } from '../../types/transit'

interface RouteDetailsTabProps {
  routeData?: BusRouteInfo | null
  isLoading?: boolean
  error?: ApiErrorBody | null
  busCode: string
}

interface UrlRowProps {
  label: string
  hint: string
  value?: string
  copyKey: string
  copiedKey: string | null
  onCopy: (text: string, key: string) => void
}

function UrlRow({ label, hint, value, copyKey, copiedKey, onCopy }: UrlRowProps) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-xs text-nordic-500 font-medium gap-2">
        <span className="flex items-center gap-1.5">
          <Globe className="w-3.5 h-3.5" /> {label}
        </span>
        <span className="font-mono text-[11px] text-nordic-400 truncate">{hint}</span>
      </div>
      <div className="flex items-center gap-2 p-2.5 rounded-md bg-nordic-50 dark:bg-nordic-950/70 border border-nordic-200 dark:border-nordic-800 font-mono text-xs">
        <input
          type="text"
          readOnly
          aria-label={label}
          value={value || 'Endpoint not captured'}
          className="bg-transparent flex-1 min-w-0 text-nordic-700 dark:text-nordic-300 focus:outline-none truncate"
        />
        {value && (
          <>
            <button
              type="button"
              onClick={() => onCopy(value, copyKey)}
              className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
              title="Copy URL"
              aria-label={`Copy ${label}`}
            >
              {copiedKey === copyKey ? <Check className="w-3.5 h-3.5 text-pine-500" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
            <a
              href={value}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
              title="Open in new tab"
              aria-label={`Open ${label} in a new tab`}
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </>
        )}
      </div>
    </div>
  )
}

export function RouteDetailsTab({ routeData, isLoading = false, error = null, busCode }: RouteDetailsTabProps) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  const handleCopy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedKey(key)
      setTimeout(() => setCopiedKey(null), 2000)
    } catch {
      // Clipboard unavailable (insecure context or permission denied).
    }
  }

  if (isLoading) {
    return (
      <div
        className="p-6 rounded-lg border border-nordic-200 dark:border-nordic-800 bg-white dark:bg-nordic-900 space-y-4"
        aria-busy="true"
        data-testid="route-loading"
      >
        <p className="text-xs text-nordic-500">
          Discovering the route schedule with a headless browser — this can take up to half a minute the first time,
          then it is cached.
        </p>
        <div className="animate-pulse space-y-3">
          <div className="h-5 w-48 bg-nordic-200 dark:bg-nordic-800 rounded" />
          <div className="h-10 bg-nordic-100 dark:bg-nordic-800/60 rounded" />
          <div className="h-10 bg-nordic-100 dark:bg-nordic-800/60 rounded" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div
        className="p-5 rounded-lg border border-amber-200 dark:border-amber-900/60 bg-amber-50/60 dark:bg-amber-950/20 flex items-start gap-3"
        data-testid="route-error"
      >
        <AlertCircle className="w-4 h-4 text-amber-600 mt-0.5 shrink-0" />
        <div className="text-xs space-y-1">
          <p className="font-semibold text-nordic-900 dark:text-white">Route schedule unavailable ({error.code})</p>
          <p className="text-nordic-600 dark:text-nordic-300">
            {error.message}. Live tracking is unaffected; the schedule is discovered separately.
          </p>
        </div>
      </div>
    )
  }

  if (!routeData) {
    return null
  }

  return (
    <div className="bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 rounded-lg p-5 sm:p-6 shadow-sm space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2 pb-4 border-b border-nordic-100 dark:border-nordic-800/80">
        <div className="flex items-center gap-2.5">
          <Layers className="w-5 h-5 text-frost-600 dark:text-frost-400" />
          <h3 className="font-semibold text-base text-nordic-900 dark:text-white">
            Discovered Route Endpoints for {busCode}
          </h3>
        </div>
        <span className="text-xs font-mono text-nordic-400">Headless Chromium Interception</span>
      </div>

      <div className="space-y-3.5" data-testid="route-endpoints">
        <UrlRow
          label="Scheduler Live Route URL"
          hint="routedetailslive"
          value={routeData.route_url}
          copyKey="route"
          copiedKey={copiedKey}
          onCopy={handleCopy}
        />
        <UrlRow
          label="Live Tracking Telemetry URL"
          hint="dashboard/chatbot/raw"
          value={routeData.tracking_url}
          copyKey="track"
          copiedKey={copiedKey}
          onCopy={handleCopy}
        />
      </div>
    </div>
  )
}
