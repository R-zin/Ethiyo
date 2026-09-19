package models

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// validBusCodePattern allows alphanumeric chars, hyphens, and underscores, between 1 and 64 characters.
	validBusCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

	// ErrBusCodeRequired indicates an empty bus code was supplied.
	ErrBusCodeRequired = errors.New("buscode is required")

	// ErrInvalidBusCodeFormat indicates the bus code contains invalid characters or exceeds length limits.
	ErrInvalidBusCodeFormat = errors.New("buscode must be alphanumeric (hyphens and underscores allowed) and between 1 and 64 characters")
)

// ValidateBusCode validates the bus code parameter against safe characters and length constraints.
func ValidateBusCode(busCode string) (string, error) {
	trimmed := strings.TrimSpace(busCode)
	if trimmed == "" {
		return "", ErrBusCodeRequired
	}
	if !validBusCodePattern.MatchString(trimmed) {
		return "", ErrInvalidBusCodeFormat
	}
	return trimmed, nil
}

// BusTrackingInfo contains the live tracking URL for a given bus code.
type BusTrackingInfo struct {
	BusCode     string `json:"bus_code"`
	TrackingURL string `json:"tracking_url"`
}

// BusRouteInfo contains the tracking URL and live route details URL discovered for a bus code.
type BusRouteInfo struct {
	BusCode     string `json:"bus_code"`
	TrackingURL string `json:"tracking_url"`
	RouteURL    string `json:"route_url"`
}
