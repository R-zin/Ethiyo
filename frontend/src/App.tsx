import { useState, useEffect, useCallback } from 'react'
import { Header } from './components/common/Header'
import { SearchBar } from './components/tracking/SearchBar'
import { LiveTrackerCard } from './components/tracking/LiveTrackerCard'
import { TransitMap } from './components/tracking/TransitMap'
import { RouteDetailsTab } from './components/tracking/RouteDetailsTab'
import { ProximityCard } from './components/tracking/ProximityCard'
import { FavoritesPanel } from './components/tracking/FavoritesPanel'
import { AlertsCard, AlertBanners } from './components/tracking/AlertsCard'
import { EmptyState } from './components/states/EmptyState'
import { ErrorState } from './components/states/ErrorState'
import { SkeletonLoader } from './components/common/SkeletonLoader'
import { TransitApi } from './services/api'
import { HealthStatus } from './types/transit'
import { useTheme } from './hooks/useTheme'
import { useBusTracker } from './hooks/useBusTracker'
import { useGeolocation } from './hooks/useGeolocation'
import { useFavorites } from './hooks/useFavorites'
import { useFleet } from './hooks/useFleet'
import { useAlerts } from './hooks/useAlerts'
import { normalizeCode } from './utils/busCode'
import { Map, Layers, Download } from 'lucide-react'

function readCodeFromUrl(): string {
  return normalizeCode(new URLSearchParams(window.location.search).get('code') || '')
}

export function App() {
  const { theme, toggleTheme } = useTheme()
  const [activeCode, setActiveCode] = useState<string>(readCodeFromUrl)
  const [activeTab, setActiveTab] = useState<'map' | 'route'>('map')
  const tracker = useBusTracker(activeCode)
  const geo = useGeolocation()
  const favorites = useFavorites()
  const fleet = useFleet(favorites.favorites)

  // System health
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  const checkHealth = useCallback(async () => {
    setHealthLoading(true)
    try {
      setHealth(await TransitApi.checkHealth())
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

  const { retry } = tracker
  const handleSearch = useCallback(
    (busCode: string) => {
      const clean = normalizeCode(busCode)
      if (!clean) return
      const url = new URL(window.location.href)
      url.searchParams.set('code', clean)
      window.history.replaceState({}, '', url.toString())
      if (clean === activeCode) {
        retry() // re-searching the same bus means "try again"
      } else {
        setActiveCode(clean)
      }
    },
    [activeCode, retry]
  )

  const { tracking, error, isLoading } = tracker
  const live = tracking?.live
  const alerts = useAlerts({
    busCode: tracking?.bus_code ?? '',
    live,
    degraded: tracking?.degraded,
    user: geo.position,
  })

  return (
    <div className="min-h-full flex flex-col bg-nordic-50 dark:bg-nordic-950 text-nordic-900 dark:text-nordic-100 selection:bg-frost-500 selection:text-white">
      <Header
        theme={theme}
        onToggleTheme={toggleTheme}
        health={health}
        healthLoading={healthLoading}
        onInspectHealth={checkHealth}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8 space-y-6">
        <section aria-label="Route Search" className="max-w-2xl mx-auto w-full">
          <SearchBar onSearch={handleSearch} isLoading={isLoading} initialCode={activeCode} />
        </section>

        <FavoritesPanel
          favorites={favorites.favorites}
          entries={fleet.entries}
          mode={fleet.mode}
          activeCode={tracking?.bus_code}
          onSelect={handleSearch}
          onRemove={favorites.remove}
        />

        <AlertBanners alerts={alerts.alerts} onDismiss={alerts.dismiss} />

        <div className="pt-2">
          {isLoading && !tracking ? (
            <SkeletonLoader />
          ) : error && !tracking ? (
            <ErrorState
              code={error.code}
              message={error.message}
              busCode={activeCode}
              onRetry={retry}
              onTryAlternative={handleSearch}
            />
          ) : tracking ? (
            <div className="space-y-6">
              <LiveTrackerCard
                data={tracking}
                mode={tracker.mode}
                paused={tracker.paused}
                onTogglePaused={() => tracker.setPaused(p => !p)}
                onRefresh={tracker.refresh}
                isRefreshing={tracker.isRefreshing}
                warning={tracker.warning}
                distanceM={tracker.distanceM}
                trailPoints={tracker.trail.length}
                isFavorite={favorites.isFavorite(tracking.bus_code)}
                favoriteDisabled={favorites.isFull}
                onToggleFavorite={() => favorites.toggle(tracking.bus_code)}
              />

              <ProximityCard
                bus={live ? { latitude: live.latitude, longitude: live.longitude, speed: live.speed } : undefined}
                status={geo.status}
                position={geo.position}
                onStart={geo.start}
                onStop={geo.stop}
              />

              <AlertsCard
                busCode={tracking.bus_code}
                config={alerts.config}
                permission={alerts.permission}
                locationStatus={geo.status}
                onApproachChange={alerts.setApproachMinutes}
                onOfflineChange={alerts.setOffline}
                onEnableLocation={geo.start}
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

                <div className="flex items-center gap-3 text-xs text-nordic-400 font-mono">
                  {tracker.trail.length > 0 && (
                    <span className="flex items-center gap-1.5" data-testid="trail-export">
                      <Download className="w-3.5 h-3.5" />
                      <span className="hidden sm:inline">Trail:</span>
                      <a
                        href={TransitApi.trailExportUrl(tracking.bus_code, 'geojson')}
                        download
                        className="text-frost-600 dark:text-frost-400 hover:underline"
                        aria-label={`Download ${tracking.bus_code} trail as GeoJSON`}
                      >
                        GeoJSON
                      </a>
                      <span aria-hidden="true">·</span>
                      <a
                        href={TransitApi.trailExportUrl(tracking.bus_code, 'gpx')}
                        download
                        className="text-frost-600 dark:text-frost-400 hover:underline"
                        aria-label={`Download ${tracking.bus_code} trail as GPX`}
                      >
                        GPX
                      </a>
                    </span>
                  )}
                  <span className="hidden sm:inline">
                    {!live ? 'No GPS fix' : tracking.degraded ? 'Last known' : live.stale ? 'Stale position' : 'Live GPS'}
                  </span>
                </div>
              </div>

              <div
                role="tabpanel"
                id={activeTab === 'map' ? 'panel-map' : 'panel-route'}
                aria-labelledby={activeTab === 'map' ? 'tab-map' : 'tab-route'}
              >
                {activeTab === 'map' ? (
                  <TransitMap
                    latitude={live?.latitude}
                    longitude={live?.longitude}
                    busCode={tracking.bus_code}
                    routeName={live?.route_name}
                    speedKmh={live?.speed}
                    heading={live?.heading}
                    stale={live?.stale}
                    trail={tracker.trail}
                    userPosition={geo.position}
                  />
                ) : (
                  <RouteDetailsTab
                    routeData={tracker.route}
                    isLoading={tracker.routeLoading}
                    error={tracker.routeError}
                    busCode={tracking.bus_code}
                  />
                )}
              </div>
            </div>
          ) : (
            <EmptyState onSelectSample={handleSearch} />
          )}
        </div>
      </main>

      <footer className="border-t border-nordic-200/80 dark:border-nordic-800/80 py-6 text-xs text-nordic-400 bg-white/50 dark:bg-nordic-950/50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-3">
          <p className="flex items-center gap-1.5 font-mono">
            <span>Ethiyo Transit Platform</span>
            <span>·</span>
            <span>Nordic UI Design System</span>
          </p>
          <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-nordic-500">
            <span>REST API v1</span>
            <span>·</span>
            <span>Live SSE Streaming</span>
            <span>·</span>
            <span>Chromium Headless Interception</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
