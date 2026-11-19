package chalo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/R-zin/Ethiyo/internal/breaker"
	"github.com/R-zin/Ethiyo/internal/cache"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/telemetry"
)

// Service defines high-level operations for Chalo bus route tracking and discovery.
type Service interface {
	GetTrackingURL(ctx context.Context, busCode string) (string, error)
	GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error)
	GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error)
	GetBusHistory(ctx context.Context, busCode string) (*models.BusHistory, error)
	FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error)
	FetchPublicRoute(ctx context.Context, busCode string) (int, error)
}

// Observer receives operational signals from the service, e.g. for metrics.
// Implementations must be safe for concurrent use.
type Observer interface {
	ObserveCache(cache string, hit bool)
	ObserveUpstream(op string, d time.Duration, err error)
}

// Upstream operation names reported to Observer.ObserveUpstream.
const (
	OpLiveTracking   = "live_tracking"
	OpRouteDiscovery = "route_discovery"
	OpRouteDetails   = "route_details"
	OpPublicRoute    = "public_route"
)

// DefaultStaleAfter is how old a position may be before it is flagged stale.
// Operators report every ~30s, so several missed reports means the bus has
// most likely gone dark.
const DefaultStaleAfter = 3 * time.Minute

// Options tunes caching and telemetry. The zero value disables caching (but
// keeps request coalescing) and uses default history sizes.
type Options struct {
	// LiveCacheTTL is how long a live position is served from memory. Chalo
	// positions update every ~30s, so a few seconds absorbs polling bursts
	// without serving meaningfully old data. <= 0 disables caching.
	LiveCacheTTL time.Duration
	// RouteCacheTTL is how long a discovered route URL is reused. Discovery
	// costs a headless browser session, so this should be long. <= 0
	// disables caching.
	RouteCacheTTL   time.Duration
	CacheMaxEntries int
	HistoryPoints   int
	HistoryMaxBuses int
	StaleAfter      time.Duration
	Observer        Observer

	// LastKnownTTL is how long a bus's last good position may be served,
	// flagged degraded, while Chalo is failing. <= 0 disables the fallback.
	LastKnownTTL time.Duration
	// BreakerThreshold consecutive live-tracking failures open the circuit
	// for BreakerCooldown. Non-positive values use 5 failures / 30s.
	BreakerThreshold int
	BreakerCooldown  time.Duration
}

// ChaloService implements Service using the Chalo HTTP client and the
// headless-browser scraper, adding caching, request coalescing and motion
// telemetry on top.
type ChaloService struct {
	client     *Client
	scraper    *Scraper
	liveCache  *cache.TTL[string, *models.BusLiveInfo]
	routeCache *cache.TTL[string, *models.BusRouteInfo]
	history    *telemetry.History
	lastGood   *cache.TTL[string, *models.BusLiveInfo]
	breaker    *breaker.Breaker
	staleAfter time.Duration
	observer   Observer
	now        func() time.Time
}

// NewService creates a ChaloService with caching disabled.
func NewService(client *Client, scraper *Scraper) *ChaloService {
	return NewServiceWithOptions(client, scraper, Options{})
}

// NewServiceWithOptions creates a ChaloService with the given options.
func NewServiceWithOptions(client *Client, scraper *Scraper, opts Options) *ChaloService {
	if opts.StaleAfter <= 0 {
		opts.StaleAfter = DefaultStaleAfter
	}
	return &ChaloService{
		client:     client,
		scraper:    scraper,
		liveCache:  cache.New[string, *models.BusLiveInfo](opts.LiveCacheTTL, opts.CacheMaxEntries),
		routeCache: cache.New[string, *models.BusRouteInfo](opts.RouteCacheTTL, opts.CacheMaxEntries),
		history:    telemetry.NewHistory(opts.HistoryPoints, opts.HistoryMaxBuses),
		lastGood:   cache.New[string, *models.BusLiveInfo](opts.LastKnownTTL, opts.HistoryMaxBuses),
		breaker:    breaker.New(opts.BreakerThreshold, opts.BreakerCooldown),
		staleAfter: opts.StaleAfter,
		observer:   opts.Observer,
		now:        time.Now,
	}
}

func (s *ChaloService) observeCache(name string, hit bool) {
	if s.observer != nil {
		s.observer.ObserveCache(name, hit)
	}
}

func (s *ChaloService) observeUpstream(op string, start time.Time, err error) {
	if s.observer != nil {
		s.observer.ObserveUpstream(op, time.Since(start), err)
	}
}

// GetTrackingURL returns Chalo's live-tracking URL for a bus code. The URL is
// deterministic, but existence is confirmed by fetching AND parsing the live
// payload: Chalo answers unknown vehicles with HTTP 202 plus an error body, so
// the status code alone cannot tell a live bus from a dead URL.
func (s *ChaloService) GetTrackingURL(ctx context.Context, busCode string) (string, error) {
	info, err := s.GetBusTracking(ctx, busCode)
	if err != nil {
		return "", err
	}
	return info.TrackingURL, nil
}

// GetBusTracking returns the live position and trip context for a bus,
// enriched with heading and freshness.
func (s *ChaloService) GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	live, hit, err := s.liveCache.GetOrLoad(ctx, validCode, func(ctx context.Context) (*models.BusLiveInfo, error) {
		return s.loadLive(ctx, validCode)
	})
	s.observeCache("live", hit)
	if err != nil {
		if info, ok := s.lastKnownFallback(ctx, validCode, err); ok {
			return info, nil
		}
		slog.ErrorContext(ctx, "failed to fetch live tracking", "bus_code", validCode, "error", err)
		return nil, err
	}

	// Copy before enriching: the cached value is shared between callers.
	out := *live
	out.AgeSeconds, out.Stale = s.freshness(out.RecordedAt)

	return &models.BusTrackingInfo{
		BusCode:     validCode,
		TrackingURL: out.TrackingURL,
		Live:        &out,
	}, nil
}

// isUpstreamFailure reports whether err means Chalo itself is unhealthy. An
// unknown bus is a healthy answer, not an outage.
func isUpstreamFailure(err error) bool {
	return err != nil && !errors.Is(err, ErrBusNotFound)
}

// loadLive fetches and parses one live position from Chalo and records it in
// the motion history. It runs at most once per cache TTL per bus, and not at
// all while the circuit breaker is open.
func (s *ChaloService) loadLive(ctx context.Context, busCode string) (*models.BusLiveInfo, error) {
	var live *models.BusLiveInfo
	err := s.breaker.Do(func() error {
		slog.InfoContext(ctx, "fetching live tracking", "bus_code", busCode)
		start := time.Now()
		body, trackURL, err := s.client.FetchLiveTracking(ctx, busCode)
		if err == nil {
			live, err = ParseLiveTracking(busCode, trackURL, body)
		}
		upstreamErr := err
		if !isUpstreamFailure(err) {
			upstreamErr = nil
		}
		s.observeUpstream(OpLiveTracking, start, upstreamErr)
		return err
	}, isUpstreamFailure)
	if err != nil {
		return nil, err
	}

	live.Heading = s.history.Record(busCode, models.PositionFix{
		Latitude:   live.Latitude,
		Longitude:  live.Longitude,
		Speed:      live.Speed,
		RecordedAt: live.RecordedAt,
	})
	s.lastGood.Set(busCode, live)
	return live, nil
}

// lastKnownFallback serves the bus's last good position, flagged degraded,
// when Chalo is failing. It never masks "bus not found" (the bus really is
// gone) or a caller that gave up.
func (s *ChaloService) lastKnownFallback(ctx context.Context, busCode string, err error) (*models.BusTrackingInfo, bool) {
	if ctx.Err() != nil || !isUpstreamFailure(err) {
		return nil, false
	}
	last, ok := s.lastGood.Get(busCode)
	s.observeCache("last_known", ok)
	if !ok {
		return nil, false
	}
	out := *last
	out.AgeSeconds, out.Stale = s.freshness(out.RecordedAt)
	slog.WarnContext(ctx, "Chalo unavailable; serving last known position",
		"bus_code", busCode, "age_seconds", out.AgeSeconds, "error", err)
	return &models.BusTrackingInfo{
		BusCode:     busCode,
		TrackingURL: out.TrackingURL,
		Live:        &out,
		Degraded:    true,
	}, true
}

// UpstreamStatus reports the health of the live-tracking upstream.
func (s *ChaloService) UpstreamStatus() breaker.Status {
	return s.breaker.Status()
}

// freshness reports how old a position is (to 0.1s) and whether it is stale.
func (s *ChaloService) freshness(recordedAt time.Time) (float64, bool) {
	age := s.now().Sub(recordedAt)
	if age < 0 {
		age = 0 // vehicle clock slightly ahead of ours
	}
	return math.Round(age.Seconds()*10) / 10, age > s.staleAfter
}

// GetBusHistory returns the recent breadcrumb trail of a bus. It also fetches
// the current position (served from cache when fresh) so the trail is up to
// date and so an unknown bus yields ErrBusNotFound. A bus that has just gone
// dark still returns its last known trail.
func (s *ChaloService) GetBusHistory(ctx context.Context, busCode string) (*models.BusHistory, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	_, liveErr := s.GetBusTracking(ctx, validCode)
	points, dist := s.history.Trail(validCode)
	if liveErr != nil && (len(points) == 0 || !errors.Is(liveErr, ErrBusNotFound)) {
		return nil, liveErr
	}

	return &models.BusHistory{
		BusCode:        validCode,
		Points:         points,
		DistanceMeters: math.Round(dist),
	}, nil
}

// GetBusRoute discovers the scheduler route details URL via the headless
// browser and pairs it with the deterministic live-tracking URL. Results are
// cached because each discovery costs a browser session.
func (s *ChaloService) GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	info, hit, err := s.routeCache.GetOrLoad(ctx, validCode, func(ctx context.Context) (*models.BusRouteInfo, error) {
		slog.InfoContext(ctx, "discovering bus routes", "bus_code", validCode)
		start := time.Now()
		_, routeURL, err := s.scraper.DiscoverRoutes(ctx, validCode)
		s.observeUpstream(OpRouteDiscovery, start, err)
		if err != nil {
			return nil, err
		}
		return &models.BusRouteInfo{
			BusCode:     validCode,
			TrackingURL: s.client.LiveTrackingURL(validCode),
			RouteURL:    routeURL,
		}, nil
	})
	s.observeCache("route", hit)
	if err != nil {
		slog.ErrorContext(ctx, "failed to discover bus routes", "bus_code", validCode, "error", err)
		return nil, err
	}

	out := *info
	return &out, nil
}

// FetchRouteDetails fetches live route details from a discovered Chalo endpoint.
func (s *ChaloService) FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error) {
	slog.InfoContext(ctx, "fetching route details", "url", routeURL)
	start := time.Now()
	data, err := s.client.FetchRouteDetails(ctx, routeURL)
	if !errors.Is(err, ErrInvalidRouteURL) {
		s.observeUpstream(OpRouteDetails, start, err)
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch route details", "url", routeURL, "error", err)
		return nil, err
	}
	return data, nil
}

// FetchPublicRoute sends a lightweight GET request to the public route page.
func (s *ChaloService) FetchPublicRoute(ctx context.Context, busCode string) (int, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return 0, err
	}

	slog.InfoContext(ctx, "requesting public route page", "bus_code", validCode)
	start := time.Now()
	statusCode, err := s.client.FetchPublicRoute(ctx, validCode)
	s.observeUpstream(OpPublicRoute, start, err)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch public route", "bus_code", validCode, "error", err)
		return 0, fmt.Errorf("chalo public route error: %w", err)
	}
	return statusCode, nil
}
