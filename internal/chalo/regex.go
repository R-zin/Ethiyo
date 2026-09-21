package chalo

import "regexp"

// The discovery regexes match the API path shape rather than a hardcoded
// https://chalo.com origin, because the browser page (and thus the XHRs it
// emits) is served from the configured CHALO_BASE_URL — which may be a
// staging host, a regional mirror, or a plain-HTTP local proxy. Discovery
// therefore must not require the production chalo.com origin.
//
// The scheme prefix "https?://" is required (no bare "//" scheme-relative
// matches), and the path must appear immediately after the host, so the
// pattern cannot be satisfied by an attacker-controlled URL that merely
// embeds the path in a query parameter.
var (
	// TrackURLRegex matches Chalo live route tracking endpoints.
	TrackURLRegex = regexp.MustCompile(`^https?://[^/?]+/app/api/vasudha/track/route-live-info/[^?\s]+`)

	// RouteURLRegex matches Chalo scheduler route details live endpoints.
	RouteURLRegex = regexp.MustCompile(`^https?://[^/?]+/app/api/scheduler_v4/v4/[^/\s]+/routedetailslive(?:\?|$)`)
)

// IsTrackURL returns true if the URL matches the Chalo live tracking pattern.
func IsTrackURL(url string) bool {
	return TrackURLRegex.MatchString(url)
}

// IsRouteURL returns true if the URL matches the Chalo scheduler route details pattern.
func IsRouteURL(url string) bool {
	return RouteURLRegex.MatchString(url)
}
