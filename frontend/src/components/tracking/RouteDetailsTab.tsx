import { useState } from 'react'
import { Copy, Check, ExternalLink, Globe, Layers } from 'lucide-react'
import { BusRouteInfo } from '../../types/transit'

interface RouteDetailsTabProps {
  routeData?: BusRouteInfo | null
  isLoading?: boolean
  busCode: string
}

export function RouteDetailsTab({ routeData, isLoading = false, busCode }: RouteDetailsTabProps) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 2000)
  }

  if (isLoading) {
    return (
      <div className="p-6 rounded-lg border border-nordic-200 dark:border-nordic-800 bg-white dark:bg-nordic-900 animate-pulse space-y-4">
        <div className="h-5 w-48 bg-nordic-200 dark:bg-nordic-800 rounded" />
        <div className="h-10 bg-nordic-100 dark:bg-nordic-800/60 rounded" />
        <div className="h-10 bg-nordic-100 dark:bg-nordic-800/60 rounded" />
      </div>
    )
  }

  if (!routeData) {
    return null
  }

  return (
    <div className="bg-white dark:bg-nordic-900 border border-nordic-200/80 dark:border-nordic-800 rounded-lg p-5 sm:p-6 shadow-sm space-y-5">
      <div className="flex items-center justify-between pb-4 border-b border-nordic-100 dark:border-nordic-800/80">
        <div className="flex items-center gap-2.5">
          <Layers className="w-5 h-5 text-frost-600 dark:text-frost-400" />
          <h3 className="font-semibold text-base text-nordic-900 dark:text-white">
            Discovered Route Endpoints for {busCode}
          </h3>
        </div>
        <span className="text-xs font-mono text-nordic-400">Headless Chromium Interception</span>
      </div>

      {/* Discovered URLs */}
      <div className="space-y-3.5">
        {/* Scheduler live URL */}
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-xs text-nordic-500 font-medium">
            <span className="flex items-center gap-1.5">
              <Globe className="w-3.5 h-3.5" /> Scheduler Live Route URL
            </span>
            <span className="font-mono text-[11px] text-nordic-400">routedetailslive</span>
          </div>
          <div className="flex items-center gap-2 p-2.5 rounded-md bg-nordic-50 dark:bg-nordic-950/70 border border-nordic-200 dark:border-nordic-800 font-mono text-xs">
            <input
              type="text"
              readOnly
              value={routeData.route_url || 'Endpoint not captured'}
              className="bg-transparent flex-1 text-nordic-700 dark:text-nordic-300 focus:outline-none truncate"
            />
            {routeData.route_url && (
              <>
                <button
                  onClick={() => handleCopy(routeData.route_url, 'route')}
                  className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
                  title="Copy URL"
                  aria-label="Copy route URL"
                >
                  {copiedKey === 'route' ? (
                    <Check className="w-3.5 h-3.5 text-pine-500" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
                <a
                  href={routeData.route_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
                  title="Open in new tab"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                </a>
              </>
            )}
          </div>
        </div>

        {/* Tracking URL */}
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-xs text-nordic-500 font-medium">
            <span className="flex items-center gap-1.5">
              <Globe className="w-3.5 h-3.5" /> Live Tracking Telemetry URL
            </span>
            <span className="font-mono text-[11px] text-nordic-400">route-live-info</span>
          </div>
          <div className="flex items-center gap-2 p-2.5 rounded-md bg-nordic-50 dark:bg-nordic-950/70 border border-nordic-200 dark:border-nordic-800 font-mono text-xs">
            <input
              type="text"
              readOnly
              value={routeData.tracking_url || 'Endpoint not captured'}
              className="bg-transparent flex-1 text-nordic-700 dark:text-nordic-300 focus:outline-none truncate"
            />
            {routeData.tracking_url && (
              <>
                <button
                  onClick={() => handleCopy(routeData.tracking_url, 'track')}
                  className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
                  title="Copy URL"
                  aria-label="Copy tracking URL"
                >
                  {copiedKey === 'track' ? (
                    <Check className="w-3.5 h-3.5 text-pine-500" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
                <a
                  href={routeData.tracking_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 rounded text-nordic-400 hover:text-nordic-600 dark:hover:text-nordic-200"
                  title="Open in new tab"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                </a>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
