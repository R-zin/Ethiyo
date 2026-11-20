package models

import (
	"errors"
	"fmt"
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

	// ErrTooManyBusCodes indicates a multi-bus request listed more buses than allowed.
	ErrTooManyBusCodes = errors.New("too many bus codes in one request")
)

// MaxFleetSize bounds how many buses one multi-bus request may ask for.
const MaxFleetSize = 20

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

// ParseBusCodes parses a comma-separated list of bus codes (as used by the
// multi-bus endpoints): entries are trimmed, validated and de-duplicated,
// keeping first-seen order. At most max distinct codes are accepted.
func ParseBusCodes(raw string, max int) ([]string, error) {
	var codes []string
	seen := make(map[string]bool)
	for i, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		code, err := ValidateBusCode(part)
		if err != nil {
			return nil, fmt.Errorf("%w (entry %d)", err, i+1)
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
		if len(codes) > max {
			return nil, fmt.Errorf("%w: at most %d", ErrTooManyBusCodes, max)
		}
	}
	if len(codes) == 0 {
		return nil, ErrBusCodeRequired
	}
	return codes, nil
}

// FleetItem is one bus's result in a multi-bus response: exactly one of
// Tracking and Error is set, so one bad bus never fails the whole request.
type FleetItem struct {
	BusCode  string           `json:"bus_code"`
	Tracking *BusTrackingInfo `json:"tracking,omitempty"`
	Error    *ErrorDetail     `json:"error,omitempty"`
}

// FleetResponse lists per-bus results in request order.
type FleetResponse struct {
	Buses []FleetItem `json:"buses"`
}

// BusTrackingInfo contains the live tracking URL for a given bus code, plus
// the live position snapshot when available.
type BusTrackingInfo struct {
	BusCode     string       `json:"bus_code"`
	TrackingURL string       `json:"tracking_url"`
	Live        *BusLiveInfo `json:"live,omitempty"`
	// Degraded is true when Chalo could not be reached and Live is the last
	// known position rather than a fresh one.
	Degraded bool `json:"degraded,omitempty"`
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
