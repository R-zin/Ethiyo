package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

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
