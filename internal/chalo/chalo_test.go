package chalo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
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
		// Origin-agnostic: discovery must work against any configured base
		// URL (staging, mirrors, local proxies), not just production chalo.com.
		{"https://other.com/app/api/vasudha/track/route-live-info/12345", true},
		{"http://127.0.0.1:9000/app/api/vasudha/track/route-live-info/12345", true},
		{"https://chalo.com/app/api/other/endpoint", false},
		{"ftp://chalo.com/app/api/vasudha/track/route-live-info/12345", false},
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

func TestRegexEdgeCases(t *testing.T) {
	// Scheme-less "//host/..." URLs (e.g. url.URL.String() of scheme-relative
	// inputs) must be rejected by both matchers: FindString semantics would
	// otherwise locate the pattern anywhere inside the string.
	if IsTrackURL("//chalo.com/app/api/vasudha/track/route-live-info/1") {
		t.Errorf("IsTrackURL should reject scheme-relative URLs")
	}
	if IsRouteURL("//chalo.com/app/api/scheduler_v4/v4/c/routedetailslive?x=1") {
		t.Errorf("IsRouteURL should reject scheme-relative URLs")
	}
	if IsTrackURL("https://evil.com/redir?u=https://chalo.com/app/api/vasudha/track/route-live-info/1") {
		t.Errorf("IsTrackURL should reject URLs only containing the pattern in a parameter")
	}
}

func TestFetchRouteDetailsAllowsConfiguredHTTPBaseURL(t *testing.T) {
	// A deployment fronted by a local reverse proxy may legitimately use a
	// plain-HTTP loopback CHALO_BASE_URL. The SSRF guard must accept URLs on
	// the configured host regardless of scheme, while still rejecting
	// non-loopback plain HTTP.
	var requestedURL string
	client := NewClient("http://127.0.0.1:9000", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			requestedURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		},
	})

	apiURL := "http://127.0.0.1:9000/app/api/scheduler_v4/v4/city/routedetailslive?id=1"
	if _, err := client.FetchRouteDetails(context.Background(), apiURL); err != nil {
		t.Fatalf("expected configured HTTP base URL to be allowed, got: %v", err)
	}
	if requestedURL != apiURL {
		t.Errorf("expected request to %q, got %q", apiURL, requestedURL)
	}

	// Plain HTTP to a non-loopback host must still be rejected.
	_, err := client.FetchRouteDetails(context.Background(), "http://169.254.169.254/app/api/x")
	if !errors.Is(err, ErrInvalidRouteURL) {
		t.Errorf("expected ErrInvalidRouteURL for plain-HTTP metadata host, got %v", err)
	}
}

func TestFetchRouteDetailsRejectsMalformedAndMismatchingURLs(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)

	cases := map[string]string{
		"lookalike host":       "https://chalo.com.evil.com/app/api/scheduler_v4/v4/c/routedetailslive?id=1",
		"userinfo trick":       "https://chalo.com@evil.com/app/api/scheduler_v4/v4/c/routedetailslive?id=1",
		"non-chalo https host": "https://example.com/app/api/scheduler_v4/v4/c/routedetailslive?id=1",
		"non-API path":         "https://chalo.com/app/public-route/500",
		"control character":    "https://chalo.com/app/api/\x7f/x",
	}
	for name, u := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := client.FetchRouteDetails(context.Background(), u); !errors.Is(err, ErrInvalidRouteURL) {
				t.Errorf("FetchRouteDetails(%q): expected ErrInvalidRouteURL, got %v", u, err)
			}
		})
	}
}

func TestFetchRouteDetailsUpstreamErrorStatus(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("boom")),
			}, nil
		},
	})

	_, err := client.FetchRouteDetails(context.Background(), "https://chalo.com/app/api/scheduler_v4/v4/c/routedetailslive?id=1")
	if !errors.Is(err, ErrUpstreamService) {
		t.Errorf("expected ErrUpstreamService for 500 response, got %v", err)
	}
}

func TestFetchRouteDetailsOversizedBody(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(make([]byte, 6*1024*1024))),
			}, nil
		},
	})

	_, err := client.FetchRouteDetails(context.Background(), "https://chalo.com/app/api/scheduler_v4/v4/c/routedetailslive?id=1")
	if err == nil {
		t.Fatalf("expected error for body exceeding the 5MB limit")
	}
}

func TestServiceValidationAndErrorPropagation(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	svc := NewService(client, NewScraper(nil, "https://chalo.com", 5*time.Second))

	if _, err := svc.GetTrackingURL(context.Background(), "bad$$code"); err == nil {
		t.Errorf("expected validation error from GetTrackingURL")
	}
	if _, err := svc.GetBusRoute(context.Background(), "  "); err == nil {
		t.Errorf("expected validation error from GetBusRoute")
	}
	if _, err := svc.FetchPublicRoute(context.Background(), "bad$$code"); err == nil {
		t.Errorf("expected validation error from FetchPublicRoute")
	}

	// FetchRouteDetails must surface client errors (e.g. SSRF rejection).
	if _, err := svc.FetchRouteDetails(context.Background(), "https://evil.com/x"); !errors.Is(err, ErrInvalidRouteURL) {
		t.Errorf("expected ErrInvalidRouteURL from FetchRouteDetails, got %v", err)
	}
}

// livePayload is a trimmed replica of Chalo's real vehicle-tracking response
// (captured from KS602), with the fields ParseLiveTracking consumes.
const livePayload = `{
  "gpsData": {
    "vehicleCode": "KL15A3159",
    "operator": {"name": "KSRTC"},
    "city": {"liveTracking": true}
  },
  "sessionData": {
    "data": {
      "currentInfo": {"lt": 11.2059, "ln": 75.81147, "pSp": 12.5, "tS": 1789836570000},
      "nextStop": {"name": "Cheruvannur Koyas"},
      "previousStop": {"name": "Meenchanda Bypass"},
      "routeDetails": {"routeName": "Kannur-Punalur"}
    }
  }
}`

func TestFetchLiveTrackingSuccess(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			want := "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KS602"
			if req.URL.String() != want {
				t.Errorf("unexpected URL: got %q want %q", req.URL.String(), want)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(livePayload)),
			}, nil
		},
	})

	body, trackURL, err := client.FetchLiveTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("FetchLiveTracking: %v", err)
	}
	if trackURL == "" || len(body) == 0 {
		t.Errorf("expected body and tracking URL, got body=%dB url=%q", len(body), trackURL)
	}
}

func TestFetchLiveTrackingEscapesBusCode(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	var gotURL string
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(livePayload)),
			}, nil
		},
	})
	if _, _, err := client.FetchLiveTracking(context.Background(), "KA 01"); err != nil {
		t.Fatalf("FetchLiveTracking: %v", err)
	}
	if gotURL != "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KA+01" &&
		gotURL != "https://chalo.com/app/api/dashboard/chatbot/raw?vehicleNo=KA%2001" {
		t.Errorf("bus code not query-escaped: %q", gotURL)
	}
}

// Chalo answers this endpoint with HTTP 202 for BOTH known and unknown
// vehicles, so FetchLiveTracking returns the body and lets ParseLiveTracking
// decide. TestFetchLiveTrackingSuccess uses HTTP 200; this pins the real 202
// path for a known vehicle.
func TestFetchLiveTrackingKnownVehicleHTTP202(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusAccepted, // 202 is normal here, not an error
				Body:       io.NopCloser(bytes.NewBufferString(livePayload)),
			}, nil
		},
	})

	body, trackURL, err := client.FetchLiveTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("FetchLiveTracking(202): %v", err)
	}
	if trackURL == "" {
		t.Errorf("expected tracking URL on 202")
	}
	// And the body must parse into a live position.
	if _, err := ParseLiveTracking("KS602", trackURL, body); err != nil {
		t.Errorf("ParseLiveTracking on 202 body: %v", err)
	}
}

func TestFetchLiveTrackingUnknownVehicle(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			// Unknown vehicles also get HTTP 202, but with an error body.
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":"gps data is not present for ZZZZZ9 vehicle"}`)),
			}, nil
		},
	})

	body, trackURL, err := client.FetchLiveTracking(context.Background(), "ZZZZZ9")
	if err != nil {
		t.Fatalf("FetchLiveTracking: %v", err)
	}
	// The 202 + error body yields ErrBusNotFound only after parsing.
	if _, err := ParseLiveTracking("ZZZZZ9", trackURL, body); !errors.Is(err, ErrBusNotFound) {
		t.Errorf("expected ErrBusNotFound after parsing, got %v", err)
	}
}

func TestFetchLiveTrackingUpstreamError(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(bytes.NewBufferString("bad gateway")),
			}, nil
		},
	})

	_, _, err := client.FetchLiveTracking(context.Background(), "KS602")
	if !errors.Is(err, ErrUpstreamService) {
		t.Errorf("expected ErrUpstreamService, got %v", err)
	}
}

func TestParseLiveTrackingValid(t *testing.T) {
	info, err := ParseLiveTracking("KS602", "https://chalo.com/track", []byte(livePayload))
	if err != nil {
		t.Fatalf("ParseLiveTracking: %v", err)
	}
	if info.BusCode != "KS602" {
		t.Errorf("BusCode = %q", info.BusCode)
	}
	if info.VehicleCode != "KL15A3159" || info.Operator != "KSRTC" {
		t.Errorf("vehicle/operator = %q/%q", info.VehicleCode, info.Operator)
	}
	if info.Latitude != 11.2059 || info.Longitude != 75.81147 {
		t.Errorf("position = %v,%v", info.Latitude, info.Longitude)
	}
	if info.Speed != 12.5 {
		t.Errorf("speed = %v", info.Speed)
	}
	if info.RecordedAt.UnixMilli() != 1789836570000 {
		t.Errorf("recordedAt = %v", info.RecordedAt)
	}
	if info.NextStop != "Cheruvannur Koyas" || info.PreviousStop != "Meenchanda Bypass" {
		t.Errorf("stops = %q/%q", info.NextStop, info.PreviousStop)
	}
	if info.RouteName != "Kannur-Punalur" {
		t.Errorf("route = %q", info.RouteName)
	}
	if !info.LiveTracking {
		t.Errorf("expected LiveTracking true")
	}
	if info.TrackingURL != "https://chalo.com/track" {
		t.Errorf("trackingURL = %q", info.TrackingURL)
	}
}

func TestParseLiveTrackingNoPosition(t *testing.T) {
	cases := map[string]string{
		"zero timestamp":   `{"sessionData":{"data":{"currentInfo":{"lt":1.5,"ln":2.5,"tS":0}}}}`,
		"zero coordinates": `{"sessionData":{"data":{"currentInfo":{"lt":0,"ln":0,"tS":1789836570000}}}}`,
		"empty payload":    `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseLiveTracking("X", "u", []byte(body))
			if !errors.Is(err, ErrBusNotFound) {
				t.Errorf("expected ErrBusNotFound, got %v", err)
			}
		})
	}
}

func TestParseLiveTrackingInvalidJSON(t *testing.T) {
	_, err := ParseLiveTracking("X", "u", []byte("not json"))
	if err == nil || errors.Is(err, ErrBusNotFound) {
		t.Errorf("expected decode error (not ErrBusNotFound), got %v", err)
	}
}

func TestServiceGetBusTrackingEndToEnd(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(livePayload)),
			}, nil
		},
	})
	svc := NewService(client, NewScraper(nil, "https://chalo.com", 5*time.Second))

	info, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("GetBusTracking: %v", err)
	}
	if info.BusCode != "KS602" || info.Live == nil {
		t.Fatalf("expected BusCode KS602 with live data, got %+v", info)
	}
	if info.Live.VehicleCode != "KL15A3159" {
		t.Errorf("live vehicle = %q", info.Live.VehicleCode)
	}
	if info.TrackingURL == "" {
		t.Errorf("expected tracking URL")
	}

	// Unknown vehicle: Chalo returns 202 + error body; the service surfaces
	// ErrBusNotFound after parsing finds no position.
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":"gps data is not present for ZZZZZ9 vehicle"}`)),
			}, nil
		},
	})
	_, err = svc.GetBusTracking(context.Background(), "ZZZZZ9")
	if !errors.Is(err, ErrBusNotFound) {
		t.Errorf("expected ErrBusNotFound, got %v", err)
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

func TestChaloService(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://chalo.com/app/public-route/12345" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("ok")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"status":"active"}`)),
			}, nil
		},
	})

	svc := NewService(client, nil)

	// Test bus code validation rejection across methods
	if _, err := svc.GetTrackingURL(context.Background(), "invalid$$code"); err == nil {
		t.Errorf("expected validation error on GetTrackingURL")
	}
	if _, err := svc.GetBusTracking(context.Background(), ""); err == nil {
		t.Errorf("expected validation error on GetBusTracking")
	}
	if _, err := svc.GetBusRoute(context.Background(), "bad/code"); err == nil {
		t.Errorf("expected validation error on GetBusRoute")
	}
	if _, err := svc.FetchPublicRoute(context.Background(), ""); err == nil {
		t.Errorf("expected validation error on FetchPublicRoute")
	}

	// Test valid FetchPublicRoute call
	statusCode, err := svc.FetchPublicRoute(context.Background(), "12345")
	if err != nil || statusCode != http.StatusOK {
		t.Errorf("expected 200 from FetchPublicRoute, got status %d, err %v", statusCode, err)
	}

	// Test valid FetchRouteDetails call
	data, err := svc.FetchRouteDetails(context.Background(), "https://chalo.com/app/api/route/test")
	if err != nil || string(data) != `{"status":"active"}` {
		t.Errorf("expected valid data from FetchRouteDetails, got %s, err %v", string(data), err)
	}
}

func TestRouteCacheOperations(t *testing.T) {
	cache := NewRouteCache(50*time.Millisecond, 50*time.Millisecond)

	// Case 1: Route caching and TTL expiration
	route := &models.BusRouteInfo{
		BusCode:     "DL1PC1234",
		TrackingURL: "https://chalo.com/track/1234",
		RouteURL:    "https://chalo.com/route/1234",
	}

	if _, ok := cache.GetRoute("DL1PC1234"); ok {
		t.Fatalf("expected cache miss initially")
	}

	cache.SetRoute("DL1PC1234", route)
	cachedRoute, ok := cache.GetRoute("DL1PC1234")
	if !ok || cachedRoute.RouteURL != route.RouteURL {
		t.Fatalf("expected cache hit with matching route, got ok=%v, route=%v", ok, cachedRoute)
	}

	// Wait for TTL expiration
	time.Sleep(60 * time.Millisecond)
	if _, ok := cache.GetRoute("DL1PC1234"); ok {
		t.Errorf("expected route cache entry to be expired")
	}

	// Case 2: Tracking caching and TTL expiration
	tracking := &models.BusTrackingInfo{
		BusCode:     "DL1PC5678",
		TrackingURL: "https://chalo.com/track/5678",
	}

	if _, ok := cache.GetTracking("DL1PC5678"); ok {
		t.Fatalf("expected tracking cache miss initially")
	}

	cache.SetTracking("DL1PC5678", tracking)
	cachedTracking, ok := cache.GetTracking("DL1PC5678")
	if !ok || cachedTracking.TrackingURL != tracking.TrackingURL {
		t.Fatalf("expected cache hit with matching tracking info, got ok=%v", ok)
	}

	time.Sleep(60 * time.Millisecond)
	if _, ok := cache.GetTracking("DL1PC5678"); ok {
		t.Errorf("expected tracking cache entry to be expired")
	}

	// Case 3: Clear
	cache.SetRoute("DL1PC1111", route, 1*time.Minute)
	cache.SetTracking("DL1PC2222", tracking, 1*time.Minute)
	cache.Clear()

	if _, ok := cache.GetRoute("DL1PC1111"); ok {
		t.Errorf("expected route to be cleared")
	}
	if _, ok := cache.GetTracking("DL1PC2222"); ok {
		t.Errorf("expected tracking to be cleared")
	}
}

func TestChaloServiceCacheIntegration(t *testing.T) {
	callCount := 0
	client := NewClient("https://chalo.com", 5*time.Second)
	client.SetHTTPClient(&mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(livePayload)),
			}, nil
		},
	})

	svc := NewService(client, nil)

	// First call should invoke upstream client
	info1, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("GetBusTracking call 1 failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected callCount 1, got %d", callCount)
	}

	// Second call should hit the cache and not increment callCount
	info2, err := svc.GetBusTracking(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("GetBusTracking call 2 failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected callCount to remain 1 on cache hit, got %d", callCount)
	}
	if info1.BusCode != info2.BusCode {
		t.Errorf("expected matching cached data")
	}

	// GetTrackingURL should also hit cached tracking
	url, err := svc.GetTrackingURL(context.Background(), "KS602")
	if err != nil {
		t.Fatalf("GetTrackingURL failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected callCount to remain 1 when GetTrackingURL uses cache, got %d", callCount)
	}
	if url != info1.TrackingURL {
		t.Errorf("expected matching tracking URL: %q vs %q", url, info1.TrackingURL)
	}
}

func TestChaloServiceScraperDisabled(t *testing.T) {
	client := NewClient("https://chalo.com", 5*time.Second)
	svc := NewService(client, nil, WithScraperEnabled(false))

	if svc.ScraperEnabled() {
		t.Errorf("expected ScraperEnabled to be false")
	}

	// When scraper is disabled, GetBusRoute must return ErrScraperDisabled
	_, err := svc.GetBusRoute(context.Background(), "DL1PC1234")
	if !errors.Is(err, ErrScraperDisabled) {
		t.Fatalf("expected ErrScraperDisabled when scraper is disabled, got: %v", err)
	}

	// But if route is already cached, it should return cached route even if scraper is disabled
	route := &models.BusRouteInfo{
		BusCode:     "DL1PC1234",
		TrackingURL: "https://chalo.com/track/1234",
		RouteURL:    "https://chalo.com/route/1234",
	}
	svc.Cache().SetRoute("DL1PC1234", route)

	cached, err := svc.GetBusRoute(context.Background(), "DL1PC1234")
	if err != nil {
		t.Fatalf("unexpected error fetching cached route: %v", err)
	}
	if cached.RouteURL != route.RouteURL {
		t.Errorf("expected route %q, got %q", route.RouteURL, cached.RouteURL)
	}
}
