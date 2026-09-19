package chalo

import "regexp"

var (
	// TrackURLRegex matches Chalo live route tracking endpoints.
	TrackURLRegex = regexp.MustCompile(`https://chalo\.com/app/api/vasudha/track/route-live-info/[^?]+`)

	// RouteURLRegex matches Chalo scheduler route details live endpoints.
	RouteURLRegex = regexp.MustCompile(`https://chalo\.com/app/api/scheduler_v4/v4/[^/]+/routedetailslive\?`)
)

// IsTrackURL returns true if the URL matches the Chalo live tracking pattern.
func IsTrackURL(url string) bool {
	return TrackURLRegex.MatchString(url)
}

// IsRouteURL returns true if the URL matches the Chalo scheduler route details pattern.
func IsRouteURL(url string) bool {
	return RouteURLRegex.MatchString(url)
}
