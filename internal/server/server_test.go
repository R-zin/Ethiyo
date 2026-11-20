package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/apidocs"
	"github.com/R-zin/Ethiyo/internal/config"
)

func TestServerInitializationAndRoutes(t *testing.T) {
	cfg := &config.Config{
		Port:                  "8080",
		Env:                   "development",
		ChaloBaseURL:          "https://chalo.com",
		RequestTimeout:        5 * time.Second,
		BrowserTimeout:        5 * time.Second,
		BrowserMaxConcurrency: 2,
		RateLimitRPS:          10,
		RateLimitBurst:        20,
		CORSAllowedOrigins:    []string{"*"},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	engine := srv.Engine()

	// Test /health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", w.Code)
	}

	// Test /api/v1/health endpoint
	reqV1 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	wV1 := httptest.NewRecorder()
	engine.ServeHTTP(wV1, reqV1)

	if wV1.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/health, got %d", wV1.Code)
	}

	// Test legacy /trackroute endpoint with missing buscode (should return 400, not crash)
	reqLegacy := httptest.NewRequest(http.MethodGet, "/trackroute", nil)
	wLegacy := httptest.NewRecorder()
	engine.ServeHTTP(wLegacy, reqLegacy)

	if wLegacy.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty buscode on /trackroute, got %d", wLegacy.Code)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Port:                  freePort(t),
		Env:                   "test",
		ChaloBaseURL:          "https://chalo.com",
		RequestTimeout:        5 * time.Second,
		BrowserTimeout:        5 * time.Second,
		BrowserMaxConcurrency: 2,
		RateLimitRPS:          10,
		RateLimitBurst:        20,
		CORSAllowedOrigins:    []string{"*"},
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func TestServerStartAndGracefulShutdown(t *testing.T) {
	srv := newTestServer(t)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	addr := "127.0.0.1:" + srv.cfg.Port
	var resp *http.Response
	var err error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + addr + "/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never became ready: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health returned %d", resp.StatusCode)
	}

	// Graceful shutdown via SIGTERM.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned error on graceful shutdown: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("server did not shut down within timeout")
	}
}

func TestServerStartFailsOnPortConflict(t *testing.T) {
	srv1 := newTestServer(t)
	errCh := make(chan error, 1)
	go func() { errCh <- srv1.Start() }()

	// Wait until srv1's port is accepting connections.
	addr := "127.0.0.1:" + srv1.cfg.Port
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	// A second server on the same port must fail to start.
	cfg := &config.Config{
		Port:                  srv1.cfg.Port,
		Env:                   "test",
		ChaloBaseURL:          "https://chalo.com",
		RequestTimeout:        5 * time.Second,
		BrowserTimeout:        5 * time.Second,
		BrowserMaxConcurrency: 1,
		RateLimitRPS:          10,
		RateLimitBurst:        20,
	}
	srv2, err := New(cfg)
	if err != nil {
		t.Fatalf("New srv2: %v", err)
	}
	if err := srv2.Start(); err == nil {
		t.Fatalf("expected port-conflict error from second server")
	} else if !strings.Contains(err.Error(), "server error") && !errors.Is(err, syscall.EADDRINUSE) {
		t.Logf("port conflict error: %v", err)
	}

	// Clean up srv1.
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	select {
	case <-errCh:
	case <-time.After(15 * time.Second):
		t.Fatalf("srv1 did not shut down")
	}
}

func TestServerRejectsOversizedBody(t *testing.T) {
	srv := newTestServer(t)
	engine := srv.Engine()

	body := strings.NewReader(strings.Repeat("x", 3*1024*1024)) // exceeds 2MB limit
	req := httptest.NewRequest(http.MethodPost, "/health", body)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("expected oversized body to be rejected, got 200")
	}
}

func TestServerMiddlewareHeadersPresent(t *testing.T) {
	srv := newTestServer(t)
	engine := srv.Engine()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "X-Request-Id"} {
		if w.Header().Get(h) == "" {
			t.Errorf("expected %s header on /health", h)
		}
	}
}

// newUpstreamMock serves Chalo's live-tracking endpoint (HTTP 202, as the
// real service does) for any vehicle.
func newUpstreamMock(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"gpsData":{"vehicleCode":"V1"},"sessionData":{"data":{"currentInfo":{"lt":10.5,"ln":76.2,"pSp":20,"tS":%d}}}}`,
			time.Now().UnixMilli())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func startForTest(t *testing.T, mutate func(*config.Config)) (*Server, string) {
	t.Helper()
	upstream := newUpstreamMock(t)
	cfg := &config.Config{
		Port:                  freePort(t),
		Env:                   "test",
		ChaloBaseURL:          upstream.URL,
		RequestTimeout:        5 * time.Second,
		BrowserTimeout:        5 * time.Second,
		BrowserMaxConcurrency: 1,
		RateLimitRPS:          10,
		RateLimitBurst:        20,
		CORSAllowedOrigins:    []string{"*"},
		LiveCacheTTL:          time.Second,
		StreamPollInterval:    50 * time.Millisecond,
		StreamMaxClients:      10,
		StreamHeartbeat:       time.Second,
		StreamMaxLifetime:     time.Minute,
		MetricsEnabled:        true,
	}
	if mutate != nil {
		mutate(cfg)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = srv.http.ListenAndServe() }()
	base := "http://127.0.0.1:" + cfg.Port
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never became ready: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return srv, base
}

// TestShutdownEndsLiveStreamsPromptly guards against long-lived SSE
// connections stalling graceful shutdown for the whole grace period.
func TestShutdownEndsLiveStreamsPromptly(t *testing.T) {
	srv, base := startForTest(t, nil)

	resp, err := http.Get(base + "/api/v1/bus/KS602/stream")
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	r := bufio.NewReader(resp.Body)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading stream: %v", err)
		}
		if strings.HasPrefix(line, "event: position") {
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v (a live stream blocked graceful shutdown)", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("shutdown took %v with an open stream", elapsed)
	}
	if _, err := io.Copy(io.Discard, r); err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Logf("stream ended with %v", err)
	}
}

func TestMetricsEndpointExposesFeatureGauges(t *testing.T) {
	_, base := startForTest(t, nil)

	// Generate a cache miss then a hit.
	for i := 0; i < 2; i++ {
		resp, err := http.Get(base + "/api/v1/bus/KS602/track")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	for _, want := range []string{
		`ethiyo_cache_requests_total{cache="live",result="hit"} 1`,
		`ethiyo_cache_requests_total{cache="live",result="miss"} 1`,
		`ethiyo_upstream_requests_total{op="live_tracking",outcome="success"} 1`,
		`ethiyo_http_requests_total{method="GET",route="/api/v1/bus/:buscode/track",status="200"} 2`,
		"ethiyo_stream_subscribers 0",
		"ethiyo_browser_sessions_capacity 1",
		"ethiyo_upstream_circuit_open 0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}

func TestMetricsCanBeDisabled(t *testing.T) {
	_, base := startForTest(t, func(c *config.Config) { c.MetricsEnabled = false })
	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("/metrics with METRICS_ENABLED=false: status %d, want 404", resp.StatusCode)
	}
}

func TestHealthProbesBypassRateLimit(t *testing.T) {
	_, base := startForTest(t, func(c *config.Config) {
		c.RateLimitRPS = 0.001
		c.RateLimitBurst = 1
	})
	for i := 0; i < 5; i++ {
		resp, err := http.Get(base + "/health")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("health probe %d: status %d, want 200", i, resp.StatusCode)
		}
	}
}

func TestHistoryEndpointEndToEnd(t *testing.T) {
	_, base := startForTest(t, nil)
	resp, err := http.Get(base + "/api/v1/bus/KS602/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			BusCode string `json:"bus_code"`
			Points  []struct {
				Latitude float64 `json:"latitude"`
			} `json:"points"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Data.BusCode != "KS602" || len(body.Data.Points) != 1 || body.Data.Points[0].Latitude != 10.5 {
		t.Errorf("unexpected history response: %+v", body)
	}
}

// switchableUpstream serves Chalo's live endpoint and can be told to fail,
// to exercise degradation and recovery end to end.
type switchableUpstream struct {
	srv     *httptest.Server
	failing atomic.Bool
}

func newSwitchableUpstream(t *testing.T) *switchableUpstream {
	t.Helper()
	u := &switchableUpstream{}
	recorded := time.Now().Add(-20 * time.Second).UnixMilli()
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u.failing.Load() {
			http.Error(w, "upstream down", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"sessionData":{"data":{"currentInfo":{"lt":10.5,"ln":76.2,"pSp":20,"tS":%d}}}}`, recorded)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func getJSON(t *testing.T, url string, into any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if into != nil {
		_ = json.NewDecoder(resp.Body).Decode(into)
	}
	return resp.StatusCode
}

func TestChaloOutageDegradesGracefullyAndRecovers(t *testing.T) {
	up := newSwitchableUpstream(t)
	_, base := startForTest(t, func(c *config.Config) {
		c.ChaloBaseURL = up.srv.URL
		c.LiveCacheTTL = 0
		c.LastKnownTTL = time.Minute
		c.BreakerThreshold = 2
		c.BreakerCooldown = 200 * time.Millisecond
	})

	type trackResp struct {
		Data struct {
			Degraded bool `json:"degraded"`
			Live     struct {
				Latitude float64 `json:"latitude"`
			} `json:"live"`
		} `json:"data"`
	}
	type healthResp struct {
		Degraded bool `json:"degraded"`
		Upstream struct {
			State string `json:"state"`
		} `json:"upstream"`
	}

	var tr trackResp
	if code := getJSON(t, base+"/api/v1/bus/KS602/track", &tr); code != 200 || tr.Data.Degraded {
		t.Fatalf("healthy: status %d degraded %v", code, tr.Data.Degraded)
	}

	up.failing.Store(true)
	for i := 0; i < 3; i++ {
		tr = trackResp{}
		if code := getJSON(t, base+"/api/v1/bus/KS602/track", &tr); code != 200 || !tr.Data.Degraded || tr.Data.Live.Latitude != 10.5 {
			t.Fatalf("outage request %d: status %d, body %+v; want 200 with the last known position", i, code, tr)
		}
	}
	var hr healthResp
	if code := getJSON(t, base+"/health", &hr); code != 200 || !hr.Degraded || hr.Upstream.State != "open" {
		t.Errorf("health during outage: %d %+v; want 200, degraded, circuit open", code, hr)
	}
	// A bus never seen before has no fallback: a clear 503, not a hang.
	var er struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if code := getJSON(t, base+"/api/v1/bus/NEW1/track", &er); code != 503 || er.Error.Code != "UPSTREAM_UNAVAILABLE" {
		t.Errorf("unseen bus during outage: %d %s; want 503 UPSTREAM_UNAVAILABLE", code, er.Error.Code)
	}

	up.failing.Store(false)
	time.Sleep(250 * time.Millisecond) // cooldown elapses; next call is the trial
	tr = trackResp{}
	if code := getJSON(t, base+"/api/v1/bus/KS602/track", &tr); code != 200 || tr.Data.Degraded {
		t.Errorf("after recovery: status %d degraded %v; want fresh data", code, tr.Data.Degraded)
	}
	hr = healthResp{}
	getJSON(t, base+"/health", &hr)
	if hr.Degraded || hr.Upstream.State != "closed" {
		t.Errorf("health after recovery: %+v", hr)
	}
}

// TestOpenAPISpecCoversEveryV1Route keeps the published API description
// honest: adding, removing or renaming a /api/v1 route without updating
// internal/apidocs/openapi.json fails the build.
func TestOpenAPISpecCoversEveryV1Route(t *testing.T) {
	srv := newTestServer(t)
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(apidocs.Spec(), &spec); err != nil {
		t.Fatal(err)
	}

	toOpenAPI := regexp.MustCompile(`:([a-zA-Z_]+)`)
	registered := map[string]bool{}
	for _, r := range srv.Engine().Routes() {
		if strings.HasPrefix(r.Path, "/api/v1/") {
			registered[r.Method+" "+toOpenAPI.ReplaceAllString(r.Path, "{$1}")] = true
		}
	}
	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	for route := range registered {
		if !documented[route] {
			t.Errorf("route %s is not documented in openapi.json", route)
		}
	}
	for route := range documented {
		if !registered[route] {
			t.Errorf("openapi.json documents %s, but no such route is registered", route)
		}
	}

	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("GET /api/v1/openapi.json: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestFleetEndpointEndToEnd(t *testing.T) {
	_, base := startForTest(t, nil)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Buses []struct {
				BusCode  string `json:"bus_code"`
				Tracking *struct {
					Live struct {
						Latitude float64 `json:"latitude"`
					} `json:"live"`
				} `json:"tracking"`
			} `json:"buses"`
		} `json:"data"`
	}
	if code := getJSON(t, base+"/api/v1/buses/track?codes=KS602,DL1PC0001", &body); code != 200 {
		t.Fatalf("status %d", code)
	}
	if !body.Success || len(body.Data.Buses) != 2 || body.Data.Buses[1].BusCode != "DL1PC0001" ||
		body.Data.Buses[0].Tracking == nil || body.Data.Buses[0].Tracking.Live.Latitude != 10.5 {
		t.Errorf("unexpected fleet response: %+v", body)
	}
}
