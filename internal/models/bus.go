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

	// Derived motion intelligence (computed by Ethiyo, not sent by Chalo).

	// Heading is the direction of travel in degrees clockwise from true
	// north, derived from successive positions. Omitted until the bus has
	// been seen moving.
	Heading *float64 `json:"heading,omitempty"`
	// AgeSeconds is how long ago the position was recorded by the vehicle.
	AgeSeconds float64 `json:"age_seconds"`
	// Stale is true when the position is too old to be trusted as "live".
	Stale bool `json:"stale"`
}

// PositionFix is a single recorded GPS position of a bus.
type PositionFix struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Speed      float64   `json:"speed"`
	RecordedAt time.Time `json:"recorded_at"`
}

// BusHistory is the recent breadcrumb trail of a bus, oldest point first.
type BusHistory struct {
	BusCode        string        `json:"bus_code"`
	Points         []PositionFix `json:"points"`
	DistanceMeters float64       `json:"distance_m"`
}
