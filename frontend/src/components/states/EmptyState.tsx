import { Compass, ArrowRight, ShieldCheck, Zap } from 'lucide-react'

interface EmptyStateProps {
  onSelectSample: (busCode: string) => void
}

export function EmptyState({ onSelectSample }: EmptyStateProps) {
  return (
    <div className="py-12 px-6 sm:py-16 text-center rounded-lg border border-nordic-200/80 dark:border-nordic-800 bg-white/50 dark:bg-nordic-900/50 backdrop-blur-sm">
      <div className="mx-auto w-12 h-12 rounded-lg bg-frost-50 dark:bg-frost-950/60 border border-frost-200 dark:border-frost-900 flex items-center justify-center text-frost-600 dark:text-frost-400 mb-4">
        <Compass className="w-6 h-6" />
      </div>

      <h2 className="text-lg font-semibold tracking-tight text-nordic-900 dark:text-white">
        Real-Time Transit Tracking
      </h2>
      <p className="mt-1.5 text-sm text-nordic-500 dark:text-nordic-400 max-w-md mx-auto">
        Enter any Chalo transit bus code above to track live GPS telemetry, stop arrivals, and scheduler route details.
      </p>

      {/* Suggested Quick Starts */}
      <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
        <span className="text-xs font-medium text-nordic-400 uppercase tracking-wider">
          Quick Start:
        </span>
        <div className="flex flex-wrap items-center justify-center gap-2">
          {[
            { code: 'KS602', desc: 'Bangalore Live GPS' },
            { code: 'DL1PC0001', desc: 'Delhi Intercept' },
            { code: 'route_500', desc: 'Outer Ring Road' },
          ].map(sample => (
            <button
              key={sample.code}
              type="button"
              onClick={() => onSelectSample(sample.code)}
              className="px-3 py-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 bg-white dark:bg-nordic-900 hover:border-frost-400 dark:hover:border-frost-600 text-xs font-mono text-nordic-700 dark:text-nordic-200 transition-colors flex items-center gap-1.5 shadow-sm"
            >
              <span>{sample.code}</span>
              <ArrowRight className="w-3 h-3 text-frost-500" />
            </button>
          ))}
        </div>
      </div>

      {/* Feature Highlights Footer */}
      <div className="mt-12 pt-8 border-t border-nordic-100 dark:border-nordic-800/60 grid grid-cols-1 sm:grid-cols-2 gap-4 max-w-lg mx-auto text-left">
        <div className="flex items-start gap-3">
          <Zap className="w-4 h-4 text-pine-500 mt-0.5 flex-shrink-0" />
          <div>
            <h3 className="text-xs font-semibold text-nordic-800 dark:text-nordic-200">
              Direct Vehicle API
            </h3>
            <p className="text-[11px] text-nordic-500 mt-0.5">
              Instant sub-second GPS coordinate snapshots directly from transit sensors.
            </p>
          </div>
        </div>

        <div className="flex items-start gap-3">
          <ShieldCheck className="w-4 h-4 text-frost-500 mt-0.5 flex-shrink-0" />
          <div>
            <h3 className="text-xs font-semibold text-nordic-800 dark:text-nordic-200">
              Chromium Route Interception
            </h3>
            <p className="text-[11px] text-nordic-500 mt-0.5">
              Automated headless browser network listeners for live token & route discovery.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
