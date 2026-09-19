package models

import (
	"errors"
	"regexp"
	"strings"
	"time"
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

// BusTrackingInfo contains the live tracking URL for a given bus code, plus
// the live position snapshot when available.
type BusTrackingInfo struct {
	BusCode     string       `json:"bus_code"`
	TrackingURL string       `json:"tracking_url"`
	Live        *BusLiveInfo `json:"live,omitempty"`
}

// BusRouteInfo contains the tracking URL and live route details URL discovered for a bus code.
type BusRouteInfo struct {
	BusCode     string `json:"bus_code"`
	TrackingURL string `json:"tracking_url"`
	RouteURL    string `json:"route_url"`
}

// BusLiveInfo carries the live position and trip context for a bus,
// decoded from Chalo's vehicle-tracking payload.
type BusLiveInfo struct {
	BusCode      string    `json:"bus_code"`
	VehicleCode  string    `json:"vehicle_code,omitempty"`
	Operator     string    `json:"operator,omitempty"`
	RouteName    string    `json:"route_name,omitempty"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Speed        float64   `json:"speed,omitempty"`
	RecordedAt   time.Time `json:"recorded_at"`
	NextStop     string    `json:"next_stop,omitempty"`
	PreviousStop string    `json:"previous_stop,omitempty"`
	LiveTracking bool      `json:"live_tracking"`
	TrackingURL  string    `json:"tracking_url"`
}
