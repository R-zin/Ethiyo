package server

import (
	"net/http"
	"net/http/httptest"
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
