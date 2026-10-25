import { AlertCircle, RefreshCw } from 'lucide-react'

interface ErrorStateProps {
  code?: string
  message: string
  busCode?: string
  onRetry?: () => void
  onTryAlternative?: (altCode: string) => void
}

export function ErrorState({
  code = 'ERROR',
  message,
  busCode,
  onRetry,
  onTryAlternative,
}: ErrorStateProps) {
  const isNotFound = code === 'BUS_NOT_FOUND' || message.toLowerCase().includes('not found')
  const isTimeout = code === 'TIMEOUT' || code === 'CHALO_TIMEOUT'

  return (
    <div
      role="alert"
      data-testid="error-state"
      className="p-6 sm:p-8 rounded-lg border border-berry-200/80 dark:border-berry-900/60 bg-berry-50/40 dark:bg-berry-950/20 text-center max-w-xl mx-auto space-y-4 shadow-sm"
    >
      <div className="w-11 h-11 rounded-full bg-berry-100 dark:bg-berry-900/40 text-berry-600 dark:text-berry-400 flex items-center justify-center mx-auto">
        <AlertCircle className="w-5 h-5" />
      </div>

      <div className="space-y-1.5">
        <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-berry-100 dark:bg-berry-900/60 text-berry-700 dark:text-berry-300 text-xs font-mono font-medium">
          {code}
        </div>
        <h2 className="text-base font-semibold text-nordic-900 dark:text-white">
          {isNotFound
            ? `Bus '${busCode}' Not Found`
            : isTimeout
            ? 'Transit Provider Timed Out'
            : 'Unable to Retrieve Telemetry'}
        </h2>
        <p className="text-xs sm:text-sm text-nordic-600 dark:text-nordic-300 max-w-md mx-auto">
          {message}
        </p>
      </div>

      {/* Helpful Guidance */}
      {isNotFound && onTryAlternative && (
        <div className="pt-2">
          <p className="text-xs text-nordic-500 mb-2">Try one of these active test vehicles:</p>
          <div className="flex items-center justify-center gap-2">
            {['KS602', 'DL1PC0001', 'route_500'].map(alt => (
              <button
                key={alt}
                type="button"
                onClick={() => onTryAlternative(alt)}
                className="px-2.5 py-1 rounded bg-white dark:bg-nordic-900 border border-nordic-200 dark:border-nordic-800 text-xs font-mono text-frost-600 dark:text-frost-400 hover:border-frost-500 transition-colors"
              >
                {alt}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Action buttons */}
      <div className="pt-3 flex items-center justify-center gap-3">
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="px-4 py-2 rounded-md bg-nordic-900 text-white dark:bg-frost-600 dark:hover:bg-frost-500 hover:bg-nordic-800 text-xs font-medium transition-colors flex items-center gap-2"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            <span>Retry Connection</span>
          </button>
        )}
      </div>
    </div>
  )
}
