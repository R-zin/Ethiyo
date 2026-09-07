package chalo

import (
	"context"
	"fmt"
	"log/slog"

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

// ChaloService implements the Service interface using Scraper and Client.
type ChaloService struct {
	client  *Client
	scraper *Scraper
}

// NewService creates a new ChaloService.
func NewService(client *Client, scraper *Scraper) *ChaloService {
	return &ChaloService{
		client:  client,
		scraper: scraper,
	}
}

// GetTrackingURL discovers and returns the live tracking URL for a bus code.
func (s *ChaloService) GetTrackingURL(ctx context.Context, busCode string) (string, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return "", err
	}

	slog.InfoContext(ctx, "discovering tracking url", "bus_code", validCode)
	trackURL, err := s.scraper.DiscoverTrackURL(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to discover tracking url", "bus_code", validCode, "error", err)
		return "", err
	}
	return trackURL, nil
}

// GetBusTracking returns a structured BusTrackingInfo model.
func (s *ChaloService) GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
	trackURL, err := s.GetTrackingURL(ctx, busCode)
	if err != nil {
		return nil, err
	}
	return &models.BusTrackingInfo{
		BusCode:     busCode,
		TrackingURL: trackURL,
	}, nil
}

// GetBusRoute discovers both the live tracking URL and route details URL.
func (s *ChaloService) GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error) {
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "discovering bus routes", "bus_code", validCode)
	trackURL, routeURL, err := s.scraper.DiscoverRoutes(ctx, validCode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to discover bus routes", "bus_code", validCode, "error", err)
		return nil, err
	}

	return &models.BusRouteInfo{
		BusCode:     validCode,
		TrackingURL: trackURL,
		RouteURL:    routeURL,
	}, nil
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
