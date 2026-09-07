package chalo

import "errors"

var (
	// ErrBusNotFound is returned when the requested bus code does not exist on Chalo.
	ErrBusNotFound = errors.New("bus route not found")

	// ErrChaloTimeout is returned when Chalo takes too long to respond or emit network events.
	ErrChaloTimeout = errors.New("timed out waiting for Chalo response")

	// ErrDiscoveryFailed is returned when network request interception does not capture required API endpoints.
	ErrDiscoveryFailed = errors.New("failed to discover Chalo route endpoints from browser network events")

	// ErrUpstreamService is returned when the upstream Chalo API responds with an error status.
	ErrUpstreamService = errors.New("upstream Chalo service returned an error")

	// ErrInvalidRouteURL is returned when an untrusted or malformed route URL is provided for fetching.
	ErrInvalidRouteURL = errors.New("invalid route URL; must be a valid Chalo API endpoint")
)
