package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestIDMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		reqID := GetRequestID(c)
		c.String(http.StatusOK, reqID)
	})

	// Case 1: Header generated when missing
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	respID := w.Header().Get(RequestIDHeader)
	if respID == "" || respID != w.Body.String() {
		t.Errorf("expected matching Request ID in header and body, got %q", respID)
	}

	// Case 2: Header propagated when provided
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set(RequestIDHeader, "custom-trace-id")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Header().Get(RequestIDHeader) != "custom-trace-id" {
		t.Errorf("expected custom-trace-id, got %q", w2.Header().Get(RequestIDHeader))
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(RequestID(), Recovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("something went terribly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on panic, got %d", w.Code)
	}
	if !contains(w.Body.String(), "INTERNAL_SERVER_ERROR") {
		t.Errorf("expected structured JSON error, got: %s", w.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing nosniff header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("missing DENY header")
	}
}

func TestCORSMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"https://example.com"}))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Preflight OPTIONS
	reqOpt := httptest.NewRequest(http.MethodOptions, "/test", nil)
	reqOpt.Header.Set("Origin", "https://example.com")
	wOpt := httptest.NewRecorder()
	r.ServeHTTP(wOpt, reqOpt)

	if wOpt.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for OPTIONS, got %d", wOpt.Code)
	}
	if wOpt.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected allowed origin, got: %s", wOpt.Header().Get("Access-Control-Allow-Origin"))
	}

	// Disallowed origin
	reqBad := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqBad.Header.Set("Origin", "https://evil.com")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)

	if wBad.Header().Get("Access-Control-Allow-Origin") == "https://evil.com" {
		t.Errorf("disallowed origin received CORS allow header")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(1.0, 2)
	r := gin.New()
	r.Use(limiter.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Request 1: Allowed (burst capacity = 2)
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("request 1 should be allowed, got %d", w1.Code)
	}

	// Request 2: Allowed
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("request 2 should be allowed, got %d", w2.Code)
	}

	// Request 3: Blocked (exceeded burst)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("request 3 should be rate limited, got %d", w3.Code)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
