package chalo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

// mockHTTPClient implements HTTPClient interface for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestRegexMatching(t *testing.T) {
	trackTests := []struct {
		url   string
		match bool
	}{
		{"https://chalo.com/app/api/vasudha/track/route-live-info/12345", true},
		{"https://chalo.com/app/api/vasudha/track/route-live-info/route_abc?foo=bar", true},
		{"https://other.com/app/api/vasudha/track/route-live-info/12345", false},
		{"https://chalo.com/app/api/other/endpoint", false},
	}

	for _, tt := range trackTests {
		if got := IsTrackURL(tt.url); got != tt.match {
			t.Errorf("IsTrackURL(%q) = %v; want %v", tt.url, got, tt.match)
		}
	}

	routeTests := []struct {
		url   string
		match bool
	}{
		{"https://chalo.com/app/api/scheduler_v4/v4/city_1/routedetailslive?routeId=500", true},
		{"https://chalo.com/app/api/scheduler_v4/v4/delhi/routedetailslive?query=1", true},
		{"https://chalo.com/app/api/scheduler_v4/v4/city/wrongendpoint", false},
	}

	for _, tt := range routeTests {
		if got := IsRouteURL(tt.url); got != tt.match {
			t.Errorf("IsRouteURL(%q) = %v; want %v", tt.url, got, tt.match)
		}
	}
}

func TestFetchPublicRouteSuccess(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://chalo.com/app/public-route/12345" {
				t.Errorf("unexpected URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("<html><body>Public Route</body></html>")),
			}, nil
		},
	})

	code, err := client.FetchPublicRoute(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("expected status 200, got %d", code)
	}
}

func TestFetchRouteDetailsSSRFProtection(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)

	// Attempt SSRF with external domain
	_, err := client.FetchRouteDetails(context.Background(), "https://evil.com/app/api/secret")
	if !errors.Is(err, ErrInvalidRouteURL) {
		t.Errorf("expected ErrInvalidRouteURL, got %v", err)
	}

	// Attempt internal metadata service
	_, err = client.FetchRouteDetails(context.Background(), "http://169.254.169.254/latest/meta-data/")
	if !errors.Is(err, ErrInvalidRouteURL) {
		t.Errorf("expected ErrInvalidRouteURL for metadata service, got %v", err)
	}
}

func TestFetchRouteDetailsSuccess(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"routeId": "123", "status": "active"}`)),
			}, nil
		},
	})

	data, err := client.FetchRouteDetails(context.Background(), "https://chalo.com/app/api/scheduler_v4/v4/city/routedetailslive?id=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"routeId": "123", "status": "active"}` {
		t.Errorf("unexpected data: %s", string(data))
	}
}

func TestFetchRouteDetailsNotFound(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("not found")),
			}, nil
		},
	})

	_, err := client.FetchRouteDetails(context.Background(), "https://chalo.com/app/api/scheduler_v4/v4/city/routedetailslive?id=999")
	if !errors.Is(err, ErrBusNotFound) {
		t.Errorf("expected ErrBusNotFound, got %v", err)
	}
}
