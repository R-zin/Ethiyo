package chalo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

// Service defines high-level operations for Chalo bus route tracking and discovery.
type Service interface {
	GetTrackingURL(ctx context.Context, busCode string) (string, error)
	GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error)
	GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error)
	FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error)
	FetchPublicRoute(ctx context.Context, busCode string) (int, error)
}

// ChaloService implements the Service interface using Scraper, Client, and RouteCache.
type ChaloService struct {
	client         *Client
	scraper        *Scraper
	cache          *RouteCache
	scraperEnabled bool
}

// ServiceOption configures a ChaloService instance.
type ServiceOption func(*ChaloService)

// WithCache sets a custom RouteCache for ChaloService.
func WithCache(cache *RouteCache) ServiceOption {
	return func(s *ChaloService) {
		s.cache = cache
	}
}

// WithScraperEnabled configures whether the browser scraper is enabled.
func WithScraperEnabled(enabled bool) ServiceOption {
	return func(s *ChaloService) {
		s.scraperEnabled = enabled
	}
}

// NewService creates a new ChaloService with optional configuration.
func NewService(client *Client, scraper *Scraper, opts ...ServiceOption) *ChaloService {
	s := &ChaloService{
		client:         client,
		scraper:        scraper,
		cache:          NewRouteCache(15*time.Minute, 10*time.Second),
		scraperEnabled: true,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ScraperEnabled returns true if the scraper is currently enabled.
func (s *ChaloService) ScraperEnabled() bool {
	return s.scraperEnabled
}

// SetScraperEnabled dynamically enables or disables the scraper.
func (s *ChaloService) SetScraperEnabled(enabled bool) {
	s.scraperEnabled = enabled
}

// Cache returns the underlying RouteCache.
func (s *ChaloService) Cache() *RouteCache {
	return s.cache
}

// GetTrackingURL returns Chalo's live-tracking URL for a bus code. This is a
// deterministic API endpoint (no browser discovery required); existence is
// confirmed by a lightweight fetch, so an unknown vehicle surfaces as
// ErrBusNotFound rather than a broken URL.
func (s *ChaloService) GetTrackingURL(ctx context.Context, busCode string) (string, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return "", err
	}

	if s.cache != nil {
		if cached, ok := s.cache.GetTracking(validCode); ok && cached.TrackingURL != "" {
			slog.InfoContext(ctx, "cache hit for live tracking url (from tracking)", "bus_code", validCode)
			return cached.TrackingURL, nil
		}
		if cached, ok := s.cache.GetRoute(validCode); ok && cached.TrackingURL != "" {
			slog.InfoContext(ctx, "cache hit for live tracking url (from route)", "bus_code", validCode)
			return cached.TrackingURL, nil
		}
	}

	slog.InfoContext(ctx, "resolving live tracking url", "bus_code", validCode)
	_, trackURL, err := s.client.FetchLiveTracking(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to resolve live tracking url", "bus_code", validCode, "error", err)
		return "", err
	}
	return trackURL, nil
}

// GetBusTracking returns the live position and trip context for a bus.
func (s *ChaloService) GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if cached, ok := s.cache.GetTracking(validCode); ok {
			slog.InfoContext(ctx, "cache hit for live tracking", "bus_code", validCode)
			return cached, nil
		}
	}

	slog.InfoContext(ctx, "fetching live tracking", "bus_code", validCode)
	body, trackURL, err := s.client.FetchLiveTracking(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch live tracking", "bus_code", validCode, "error", err)
		return nil, err
	}

	live, err := ParseLiveTracking(validCode, trackURL, body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse live tracking", "bus_code", validCode, "error", err)
		return nil, err
	}

	info := &models.BusTrackingInfo{
		BusCode:     validCode,
		TrackingURL: live.TrackingURL,
		Live:        live,
	}

	if s.cache != nil {
		s.cache.SetTracking(validCode, info)
	}

	return info, nil
}

// GetBusRoute discovers both the live tracking URL and route details URL.
func (s *ChaloService) GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if cached, ok := s.cache.GetRoute(validCode); ok {
			slog.InfoContext(ctx, "cache hit for bus route", "bus_code", validCode)
			return cached, nil
		}
	}

	if !s.scraperEnabled || s.scraper == nil {
		return nil, ErrScraperDisabled
	}

	slog.InfoContext(ctx, "discovering bus routes", "bus_code", validCode)
	trackURL, routeURL, err := s.scraper.DiscoverRoutes(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to discover bus routes", "bus_code", validCode, "error", err)
		return nil, err
	}

	info := &models.BusRouteInfo{
		BusCode:     validCode,
		TrackingURL: trackURL,
		RouteURL:    routeURL,
	}

	if s.cache != nil {
		s.cache.SetRoute(validCode, info)
	}

	return info, nil
}

// FetchRouteDetails fetches live route details from a discovered Chalo endpoint.
func (s *ChaloService) FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error) {
	slog.InfoContext(ctx, "fetching route details", "url", routeURL)
	data, err := s.client.FetchRouteDetails(ctx, routeURL)
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
	statusCode, err := s.client.FetchPublicRoute(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch public route", "bus_code", validCode, "error", err)
		return 0, fmt.Errorf("chalo public route error: %w", err)
	}
	return statusCode, nil
}
