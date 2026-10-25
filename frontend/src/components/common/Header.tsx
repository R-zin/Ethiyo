import { Compass, Moon, Sun, Terminal } from 'lucide-react'
import { HealthStatus } from '../../types/transit'

interface HeaderProps {
  theme: 'light' | 'dark'
  onToggleTheme: () => void
  health?: HealthStatus | null
  healthLoading?: boolean
  onInspectHealth?: () => void
}

export function Header({
  theme,
  onToggleTheme,
  health,
  healthLoading = false,
  onInspectHealth,
}: HeaderProps) {
  const isHealthy = health?.status === 'healthy'

  return (
    <header className="sticky top-0 z-30 w-full border-b border-nordic-200/80 dark:border-nordic-800/80 bg-white/80 dark:bg-nordic-950/80 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        {/* Brand */}
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-9 h-9 rounded-md bg-nordic-900 text-white dark:bg-frost-500 dark:text-nordic-950 font-semibold shadow-sm">
            <Compass className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="font-semibold tracking-tight text-base text-nordic-900 dark:text-white">
                Ethiyo
              </h1>
              <span className="text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-nordic-100 text-nordic-600 dark:bg-nordic-800 dark:text-nordic-300">
                v1.2
              </span>
            </div>
            <p className="text-xs text-nordic-500 dark:text-nordic-400 hidden sm:block">
              Nordic Transit Telemetry
            </p>
          </div>
        </div>

        {/* Right Actions */}
        <div className="flex items-center gap-2 sm:gap-3">
          {/* Backend Status indicator */}
          <button
            onClick={onInspectHealth}
            title={health ? `Backend: ${health.status} (${health.latencyMs}ms)` : 'Checking backend...'}
            className="flex items-center gap-2 px-2.5 py-1.5 rounded-md border border-nordic-200 dark:border-nordic-800 hover:bg-nordic-50 dark:hover:bg-nordic-900 text-xs font-mono transition-colors text-nordic-600 dark:text-nordic-300"
            aria-label="View backend system status"
          >
            <span className="relative flex h-2 w-2">
              {isHealthy && (
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-pine-400 opacity-75" />
              )}
              <span
                className={`relative inline-flex rounded-full h-2 w-2 ${
                  healthLoading
                    ? 'bg-amber-400'
                    : isHealthy
                    ? 'bg-pine-500'
                    : 'bg-berry-500'
                }`}
              />
            </span>
            <span className="hidden sm:inline font-sans font-medium text-xs">
              {healthLoading ? 'Connecting...' : isHealthy ? 'API Online' : 'Service Down'}
            </span>
            {health?.latencyMs !== undefined && (
              <span className="text-nordic-400 text-[11px] hidden md:inline">
                {health.latencyMs}ms
              </span>
            )}
          </button>

          {/* Docs / API Shortcut */}
          <a
            href="/api/v1/health"
            target="_blank"
            rel="noopener noreferrer"
            title="Open raw JSON health endpoint"
            className="p-2 rounded-md border border-nordic-200 dark:border-nordic-800 text-nordic-600 dark:text-nordic-300 hover:bg-nordic-50 dark:hover:bg-nordic-900 transition-colors"
            aria-label="Open backend health API"
          >
            <Terminal className="w-4 h-4" />
          </a>

          {/* Theme Toggle */}
          <button
            onClick={onToggleTheme}
            className="p-2 rounded-md border border-nordic-200 dark:border-nordic-800 text-nordic-600 dark:text-nordic-300 hover:bg-nordic-50 dark:hover:bg-nordic-900 transition-colors"
            aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`}
          >
            {theme === 'light' ? (
              <Moon className="w-4 h-4 text-nordic-700" />
            ) : (
              <Sun className="w-4 h-4 text-frost-300" />
            )}
          </button>
        </div>
      </div>
    </header>
  )
}
