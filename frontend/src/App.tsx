import { useState, useEffect, useCallback } from 'react'
import { Header } from './components/common/Header'
import { SearchBar } from './components/tracking/SearchBar'
import { LiveTrackerCard } from './components/tracking/LiveTrackerCard'
import { TransitMap } from './components/tracking/TransitMap'
import { RouteDetailsTab } from './components/tracking/RouteDetailsTab'
import { EmptyState } from './components/states/EmptyState'
import { ErrorState } from './components/states/ErrorState'
import { SkeletonLoader } from './components/common/SkeletonLoader'
import { TransitApi } from './services/api'
import { BusTrackingInfo, BusRouteInfo, HealthStatus } from './types/transit'
import { useTheme } from './hooks/useTheme'
import { Map, Layers } from 'lucide-react'

export function App() {
  const { theme, toggleTheme } = useTheme()

  // Routing and tracking states
  const [activeCode, setActiveCode] = useState<string>(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('code') || ''
  })
  const [trackingData, setTrackingData] = useState<BusTrackingInfo | null>(null)
  const [routeData, setRouteData] = useState<BusRouteInfo | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [error, setError] = useState<{ code: string; message: string } | null>(null)
  const [lastUpdated, setLastUpdated] = useState<number>(Date.now())
  const [activeTab, setActiveTab] = useState<'map' | 'route'>('map')

  // System Health state
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  // Fetch health on load
  const checkHealth = useCallback(async () => {
    setHealthLoading(true)
    try {
      const res = await TransitApi.checkHealth()
      setHealth(res)
    } catch {
      setHealth({ message: 'offline', status: 'error', browser: false, latencyMs: 0 })
    } finally {
      setHealthLoading(false)
    }
  }, [])

  useEffect(() => {
    checkHealth()
    const interval = setInterval(checkHealth, 45000)
    return () => clearInterval(interval)
  }, [checkHealth])

  // Track bus function
  const handleTrackBus = useCallback(
    async (busCode: string, isSilentRefresh = false) => {
      const clean = busCode.trim().toUpperCase()
      if (!clean) return

      if (isSilentRefresh) {
        setIsRefreshing(true)
      } else {
        setIsLoading(true)
        setError(null)
      }

      // Update URL query param quietly
      const newUrl = new URL(window.location.href)
      newUrl.searchParams.set('code', clean)
      window.history.replaceState({}, '', newUrl.toString())
      setActiveCode(clean)

      try {
        // Fetch live position & tracking info
        const tracking = await TransitApi.trackBus(clean)
        setTrackingData(tracking)
        setLastUpdated(Date.now())
        setError(null)

        // Asynchronously prefetch route scheduler endpoints if not loaded
        TransitApi.getRoute(clean)
          .then(res => setRouteData(res))
          .catch(() => {
            // Non-fatal if route schedule discovery takes longer
          })
      } catch (err: any) {
        setError({
          code: err.code || 'TRACKING_FAILED',
          message: err.message || 'Unable to track bus location',
        })
        if (!isSilentRefresh) {
          setTrackingData(null)
        }
      } finally {
        setIsLoading(false)
        setIsRefreshing(false)
      }
    },
    []
  )

  // Initial load if query param was present
  useEffect(() => {
    if (activeCode) {
      handleTrackBus(activeCode)
    }
  }, [])

  return (
    <div className="min-h-full flex flex-col bg-nordic-50 dark:bg-nordic-950 text-nordic-900 dark:text-nordic-100 selection:bg-frost-500 selection:text-white">
      {/* Top Navbar */}
      <Header
        theme={theme}
        onToggleTheme={toggleTheme}
        health={health}
        healthLoading={healthLoading}
        onInspectHealth={checkHealth}
      />

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8 space-y-6">
        {/* Search & Selector */}
        <section aria-label="Route Search" className="max-w-2xl mx-auto w-full">
          <SearchBar
            onSearch={code => handleTrackBus(code)}
            isLoading={isLoading}
            initialCode={activeCode}
          />
        </section>

        {/* Content Area */}
        <div className="pt-2">
          {isLoading && !trackingData ? (
            <SkeletonLoader />
          ) : error && !trackingData ? (
            <ErrorState
              code={error.code}
              message={error.message}
              busCode={activeCode}
              onRetry={() => handleTrackBus(activeCode)}
              onTryAlternative={alt => handleTrackBus(alt)}
            />
          ) : trackingData ? (
            <div className="space-y-6">
              {/* Primary Live Telemetry Card */}
              <LiveTrackerCard
                data={trackingData}
                onRefresh={() => handleTrackBus(activeCode, true)}
                isRefreshing={isRefreshing}
                lastUpdated={lastUpdated}
              />

              {/* View Switcher Tabs (Map View vs Route Spec) */}
              <div className="flex items-center justify-between border-b border-nordic-200/80 dark:border-nordic-800 pb-2">
                <div
                  role="tablist"
                  aria-label="Transit views"
                  className="flex items-center gap-1 bg-nordic-100 dark:bg-nordic-900 p-1 rounded-lg border border-nordic-200 dark:border-nordic-800 text-xs font-medium"
                >
                  <button
                    type="button"
                    role="tab"
                    id="tab-map"
                    aria-selected={activeTab === 'map'}
                    aria-controls="panel-map"
                    onClick={() => setActiveTab('map')}
                    className={`px-3 py-1.5 rounded-md flex items-center gap-1.5 transition-colors ${
                      activeTab === 'map'
                        ? 'bg-white dark:bg-nordic-800 text-nordic-900 dark:text-white shadow-sm font-semibold'
                        : 'text-nordic-600 dark:text-nordic-400 hover:text-nordic-900 dark:hover:text-white'
                    }`}
                  >
                    <Map className="w-3.5 h-3.5" />
                    <span>Live Map View</span>
                  </button>

                  <button
                    type="button"
                    role="tab"
                    id="tab-route"
                    aria-selected={activeTab === 'route'}
                    aria-controls="panel-route"
                    onClick={() => setActiveTab('route')}
                    className={`px-3 py-1.5 rounded-md flex items-center gap-1.5 transition-colors ${
                      activeTab === 'route'
                        ? 'bg-white dark:bg-nordic-800 text-nordic-900 dark:text-white shadow-sm font-semibold'
                        : 'text-nordic-600 dark:text-nordic-400 hover:text-nordic-900 dark:hover:text-white'
                    }`}
                  >
                    <Layers className="w-3.5 h-3.5" />
                    <span>Route Endpoints & Spec</span>
                  </button>
                </div>

                <div className="text-xs text-nordic-400 hidden sm:block font-mono">
                  Coordinates: {trackingData.position?.latitude ? 'Live GPS' : 'Static Fallback'}
                </div>
              </div>

              {/* Active Tab Panel */}
              <div
                role="tabpanel"
                id={activeTab === 'map' ? 'panel-map' : 'panel-route'}
                aria-labelledby={activeTab === 'map' ? 'tab-map' : 'tab-route'}
              >
                {activeTab === 'map' ? (
                  <TransitMap
                    latitude={trackingData.position?.latitude}
                    longitude={trackingData.position?.longitude}
                    busCode={trackingData.bus_code}
                    routeName={trackingData.position?.route_name}
                    speedKmh={trackingData.position?.speed_kmh}
                  />
                ) : (
                  <RouteDetailsTab
                    routeData={routeData}
                    isLoading={!routeData && isLoading}
                    busCode={trackingData.bus_code}
                  />
                )}
              </div>
            </div>
          ) : (
            <EmptyState onSelectSample={code => handleTrackBus(code)} />
          )}
        </div>
      </main>

      {/* Nordic Minimalist Footer */}
      <footer className="border-t border-nordic-200/80 dark:border-nordic-800/80 py-6 text-xs text-nordic-400 bg-white/50 dark:bg-nordic-950/50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-3">
          <p className="flex items-center gap-1.5 font-mono">
            <span>Ethiyo Transit Platform</span>
            <span>·</span>
            <span>Nordic UI Design System</span>
          </p>
          <div className="flex items-center gap-4 text-nordic-500">
            <span>REST API v1</span>
            <span>·</span>
            <span>Chromium Headless Interception</span>
            <span>·</span>
            <span>WCAG 2.1 AA Compliant</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
