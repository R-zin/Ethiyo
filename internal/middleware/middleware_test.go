package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestRequestIDRejectsControlCharacters(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, GetRequestID(c))
	})

	// A client-supplied ID containing CR/LF/control bytes must be replaced
	// with a freshly generated hex ID, never reflected into headers/logs.
	bad := "evil-id\r\nX-Injected: true"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, bad)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := w.Header().Get(RequestIDHeader)
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("CR/LF reflected into %s response header: %q", RequestIDHeader, got)
	}
	if strings.Contains(got, "evil-id") {
		t.Errorf("unsanitized client request ID reflected: %q", got)
	}
}

func TestRequestIDRejectsOversized(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, strings.Repeat("a", 512))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := w.Header().Get(RequestIDHeader)
	if len(got) > 64 {
		t.Errorf("oversized client request ID accepted: len=%d", len(got))
	}
}

func TestCORSWildcardDoesNotSendCredentials(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"*"}))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Browsers reject "Access-Control-Allow-Origin: *" combined with
	// "Access-Control-Allow-Credentials: true". With a wildcard policy the
	// credentials header must be omitted.
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got == "true" {
		t.Errorf("wildcard CORS policy must not send Allow-Credentials: true")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected wildcard Allow-Origin, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSAllowlistSendsCredentials(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"https://example.com"}))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("allowlisted origin should receive Allow-Credentials: true")
	}
}

func TestRateLimiterEvictsStaleEntries(t *testing.T) {
	rl := NewRateLimiter(1.0, 1)
	defer rl.Stop()
	rl.ttl = 5 * time.Millisecond

	rl.allow("1.1.1.1")
	if len(rl.clients) != 1 {
		t.Fatalf("expected 1 tracked client, got %d", len(rl.clients))
	}

	time.Sleep(10 * time.Millisecond) // let the bucket go stale past ttl
	rl.cleanup()
	if got := len(rl.clients); got != 0 {
		t.Errorf("expected stale entry evicted, %d remain", got)
	}
}

func TestRateLimiterIndependentBuckets(t *testing.T) {
	rl := NewRateLimiter(0.0001, 1) // ~0 refill; effectively burst-only
	if !rl.allow("1.1.1.1") {
		t.Fatalf("first request from 1.1.1.1 should be allowed")
	}
	if rl.allow("1.1.1.1") {
		t.Fatalf("second request from 1.1.1.1 should be limited")
	}
	// A different client has its own bucket and is unaffected.
	if !rl.allow("2.2.2.2") {
		t.Fatalf("first request from 2.2.2.2 should be allowed")
	}
}

func TestLoggerDoesNotPanic(t *testing.T) {
	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusTeapot)
	})
	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", w.Code)
	}
}

func TestRequestSizeLimit(t *testing.T) {
	r := gin.New()
	r.Use(RequestSizeLimit(8))
	r.POST("/test", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("this body is too long"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized body, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("short"))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || w2.Body.String() != "short" {
		t.Errorf("small body should pass: code=%d body=%q", w2.Code, w2.Body.String())
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	rl := NewRateLimiter(1000, 64)
	done := make(chan struct{})
	for i := 0; i < 32; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				rl.allow(fmt.Sprintf("10.0.0.%d", i))
			}
		}(i)
	}
	for i := 0; i < 32; i++ {
		<-done
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
