package chalo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
)

const (
	// DefaultUserAgent is used for requests to Chalo services.
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

// HTTPClient defines the interface for executing HTTP requests against Chalo.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client handles HTTP communication with Chalo.
type Client struct {
	baseURL    string
	httpClient HTTPClient
}

// NewClient creates a new Chalo HTTP client with pooled connections.
func NewClient(baseURL string, timeout time.Duration) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// SetHTTPClient allows injecting a mock HTTP client for testing.
func (c *Client) SetHTTPClient(client HTTPClient) {
	c.httpClient = client
}

// FetchPublicRoute sends a GET request to Chalo's public route page.
func (c *Client) FetchPublicRoute(ctx context.Context, busCode string) (int, error) {
	targetURL := fmt.Sprintf("%s/app/public-route/%s", c.baseURL, busCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch public route from Chalo: %w", err)
	}
	defer resp.Body.Close()

	// Drain up to 1KB to enable connection reuse
	_, _ = io.CopyN(io.Discard, resp.Body, 1024)

	return resp.StatusCode, nil
}

// FetchRouteDetails fetches live route details JSON from a discovered Chalo endpoint.
// It enforces SSRF protection by ensuring the routeURL is an authentic Chalo API URL.
func (c *Client) FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error) {
	if err := c.validateRouteURL(routeURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, routeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request to Chalo API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrBusNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamService, resp.StatusCode)
	}

	// Limit response reading to 5MB to prevent memory exhaustion attacks
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read Chalo response body: %w", err)
	}
	if len(body) > 5*1024*1024 {
		return nil, fmt.Errorf("chalo response exceeded 5MB limit")
	}

	return body, nil
}

// FetchLiveTracking fetches the live GPS payload for a bus code directly from
// Chalo's vehicle-tracking endpoint (no browser required). It returns the raw
// payload; callers decode what they need. A vehicle with no live data yields
// ErrBusNotFound (Chalo signals this with HTTP 202 + an error body).
func (c *Client) FetchLiveTracking(ctx context.Context, busCode string) ([]byte, string, error) {
	trackURL := fmt.Sprintf("%s/app/api/dashboard/chatbot/raw?vehicleNo=%s", c.baseURL, url.QueryEscape(busCode))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trackURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch live tracking from Chalo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read Chalo response body: %w", err)
	}
	if len(body) > 5*1024*1024 {
		return nil, "", fmt.Errorf("chalo response exceeded 5MB limit")
	}

	// Chalo answers this endpoint with HTTP 202 for BOTH known vehicles (with
	// the full live payload) and unknown ones (with {"error":"gps data is not
	// present..."}). Status alone can't distinguish them, so we treat any
	// non-error status as candidate data and let ParseLiveTracking decide.
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrBusNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("%w: status %d", ErrUpstreamService, resp.StatusCode)
	}

	return body, trackURL, nil
}

// liveTrackingPayload mirrors the subset of Chalo's vehicle-tracking response
// we surface to callers. Field names match Chalo's terse JSON keys.
type liveTrackingPayload struct {
	GPSData struct {
		VehicleCode string `json:"vehicleCode"`
		Operator    struct {
			Name string `json:"name"`
		} `json:"operator"`
		City struct {
			LiveTracking bool `json:"liveTracking"`
		} `json:"city"`
	} `json:"gpsData"`
	SessionData struct {
		Data struct {
			CurrentInfo struct {
				Lat   float64 `json:"lt"`
				Lon   float64 `json:"ln"`
				Speed float64 `json:"pSp"`
				TS    int64   `json:"tS"` // epoch millis
			} `json:"currentInfo"`
			NextStop struct {
				Name string `json:"name"`
			} `json:"nextStop"`
			PreviousStop struct {
				Name string `json:"name"`
			} `json:"previousStop"`
			RouteDetails struct {
				RouteName string `json:"routeName"`
			} `json:"routeDetails"`
		} `json:"data"`
	} `json:"sessionData"`
}

// ParseLiveTracking decodes Chalo's live-tracking payload into a BusLiveInfo.
// Returns ErrBusNotFound when the payload carries no usable position.
func ParseLiveTracking(busCode, trackURL string, body []byte) (*models.BusLiveInfo, error) {
	var p liveTrackingPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("failed to decode live tracking payload: %w", err)
	}

	// Unknown/no-data vehicles come back as {"error":"gps data is not present
	// ..."} — no sessionData — or with a zero-valued currentInfo. Both mean
	// there is no usable live position.
	ci := p.SessionData.Data.CurrentInfo
	if ci.TS == 0 || (ci.Lat == 0 && ci.Lon == 0) {
		return nil, ErrBusNotFound
	}

	info := &models.BusLiveInfo{
		BusCode:      busCode,
		VehicleCode:  p.GPSData.VehicleCode,
		Operator:     p.GPSData.Operator.Name,
		RouteName:    p.SessionData.Data.RouteDetails.RouteName,
		Latitude:     ci.Lat,
		Longitude:    ci.Lon,
		Speed:        ci.Speed,
		RecordedAt:   time.UnixMilli(ci.TS),
		NextStop:     p.SessionData.Data.NextStop.Name,
		PreviousStop: p.SessionData.Data.PreviousStop.Name,
		LiveTracking: p.GPSData.City.LiveTracking,
		TrackingURL:  trackURL,
	}
	return info, nil
}

// validateRouteURL enforces SSRF protection for the route-details fetch. The
// URL must be HTTPS, live on the chalo.com host or the configured base URL's
// host, and point at the /app/api/ path prefix. URL parsing (not prefix
// matching) is used so lookalike hosts (chalo.com.evil.com), userinfo tricks
// (chalo.com@evil.com), and scheme downgrades cannot slip through.
func (c *Client) validateRouteURL(routeURL string) error {
	u, err := url.Parse(routeURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrInvalidRouteURL
	}
	if u.User != nil {
		return ErrInvalidRouteURL
	}

	allowedHosts := map[string]bool{"chalo.com": true}
	if base, err := url.Parse(c.baseURL); err == nil && base.Host != "" {
		allowedHosts[strings.ToLower(base.Host)] = true
	}

	if !allowedHosts[strings.ToLower(u.Host)] {
		return ErrInvalidRouteURL
	}
	// Plain HTTP is only acceptable for loopback targets (e.g. a local dev
	// proxy); all remote hosts must use HTTPS.
	if u.Scheme != "https" {
		host := u.Hostname()
		if u.Scheme != "http" || !isLoopbackHost(host) {
			return ErrInvalidRouteURL
		}
	}
	if !strings.HasPrefix(u.Path, "/app/api/") {
		return ErrInvalidRouteURL
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
