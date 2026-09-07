package chalo

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
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
	if !strings.HasPrefix(routeURL, "https://chalo.com/app/api/") &&
		!strings.HasPrefix(routeURL, fmt.Sprintf("%s/app/api/", c.baseURL)) {
		return nil, ErrInvalidRouteURL
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read Chalo response body: %w", err)
	}

	return body, nil
}
