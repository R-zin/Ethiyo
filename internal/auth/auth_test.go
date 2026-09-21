package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/config"
	"golang.org/x/oauth2"
)

func TestStateGenerateAndValidate(t *testing.T) {
	secret := "super-secret-key-12345"

	state, err := GenerateState(secret)
	if err != nil {
		t.Fatalf("unexpected error generating state: %v", err)
	}

	if state == "" {
		t.Fatalf("expected non-empty state string")
	}

	// Validate valid state
	err = ValidateState(state, secret, 5*time.Minute)
	if err != nil {
		t.Errorf("expected valid state to pass, got: %v", err)
	}

	// Validate with wrong secret
	err = ValidateState(state, "wrong-secret-key", 5*time.Minute)
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState for wrong secret, got: %v", err)
	}

	// Tampered state
	tamperedState := state + "tampered"
	err = ValidateState(tamperedState, secret, 5*time.Minute)
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState for tampered state, got: %v", err)
	}

	// Malformed state
	err = ValidateState("malformed-state-without-dots", secret, 5*time.Minute)
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState for malformed state, got: %v", err)
	}

	// Expired state
	err = ValidateState(state, secret, 0)
	if !errors.Is(err, ErrExpiredState) {
		t.Errorf("expected ErrExpiredState for 0 maxAge, got: %v", err)
	}
}

func TestStateUniquePerGeneration(t *testing.T) {
	secret := "super-secret-key-12345"
	s1, err := GenerateState(secret)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	s2, err := GenerateState(secret)
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	if s1 == s2 {
		t.Errorf("expected unique state strings, got identical: %q", s1)
	}
}

func TestValidateStateFutureTimestamp(t *testing.T) {
	secret := "super-secret-key-12345"

	// Craft a state with a timestamp far in the future (beyond the 1-minute
	// clock-skew allowance) — must be rejected as expired/invalid.
	future := time.Now().Add(10 * time.Minute).Unix()
	macState := buildState("abcdef0123456789", future, secret)
	if err := ValidateState(macState, secret, 30*time.Minute); !errors.Is(err, ErrExpiredState) {
		t.Errorf("expected ErrExpiredState for future timestamp, got %v", err)
	}
}

func TestValidateStateMalformedInputs(t *testing.T) {
	secret := "super-secret-key-12345"
	for _, s := range []string{
		"",
		"a.b",
		"a.b.c.d",
		"..",
		"...",
		"a.notanumber.c",
	} {
		if err := ValidateState(s, secret, time.Minute); err == nil {
			t.Errorf("ValidateState(%q): expected error, got nil", s)
		}
	}
}

// buildState constructs a state string with the exact scheme GenerateState
// uses: nonce.unixSeconds.hexHMACSHA256(nonce.unixSeconds, secret).
func buildState(nonce string, ts int64, secret string) string {
	payload := nonce + "." + strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func TestGoogleServiceDisabled(t *testing.T) {
	cfg := &config.Config{}
	svc := NewGoogleService(cfg)

	if svc.IsEnabled() {
		t.Errorf("expected svc.IsEnabled() false when credentials are empty")
	}

	_, err := svc.AuthCodeURL("test-state")
	if !errors.Is(err, ErrOAuthNotConfigured) {
		t.Errorf("expected ErrOAuthNotConfigured, got %v", err)
	}
}

func TestGoogleServiceEnabled(t *testing.T) {
	cfg := &config.Config{
		GoogleClientID:     "my-client-id",
		GoogleClientSecret: "my-client-secret",
		GoogleRedirectURL:  "https://example.com/callback",
		OAuthStateSecret:   "my-state-secret",
	}
	svc := NewGoogleService(cfg)

	if !svc.IsEnabled() {
		t.Errorf("expected svc.IsEnabled() true")
	}

	url, err := svc.AuthCodeURL("test-state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "accounts.google.com") || !strings.Contains(url, "test-state") {
		t.Errorf("unexpected auth URL: %s", url)
	}
	if !strings.Contains(url, "access_type=offline") {
		t.Errorf("expected offline access in auth URL: %s", url)
	}
}

// TestGetUserInfoAgainstMockGoogle swaps google.Endpoint for a local server
// to exercise GetUserInfo's HTTP and decoding paths.
func TestGetUserInfoAgainstMockGoogle(t *testing.T) {
	server := httptest.NewServer(nil) // placeholder; mux attached below
	server.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v2/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1","email":"u@example.com","verified_email":true,"name":"U Ser","picture":"https://x/y.png"}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"mock-access","token_type":"Bearer","expires_in":3600}`))
	})
	server = httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		GoogleClientID:     "id",
		GoogleClientSecret: "secret",
		GoogleRedirectURL:  server.URL + "/callback",
	}
	svc := NewGoogleService(cfg)
	// Point the OAuth2 endpoints at the mock.
	svc.oauthConfig.Endpoint = oauth2.Endpoint{
		AuthURL:  server.URL + "/auth",
		TokenURL: server.URL + "/token",
	}
	// Redirect userinfo fetch to the mock by rebasing the client's transport:
	// GetUserInfo uses a hardcoded URL, so instead inject a static token and
	// swap the HTTP client via context.
	token := &oauth2.Token{AccessToken: "mock-access"}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())

	// GetUserInfo hits the real Google URL; re-point it via a transport rewrite.
	transport := &rewriteTransport{base: server.Client().Transport, host: server.URL}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: transport})

	user, err := svc.GetUserInfo(ctx, token)
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if user.Email != "u@example.com" || !user.VerifiedEmail || user.Name != "U Ser" {
		t.Errorf("unexpected user info: %+v", user)
	}
}

func TestGetUserInfoNon200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{GoogleClientID: "id", GoogleClientSecret: "s", GoogleRedirectURL: "http://x/cb"}
	svc := NewGoogleService(cfg)

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, host: server.URL}})
	_, err := svc.GetUserInfo(ctx, &oauth2.Token{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 status error, got %v", err)
	}
}

// rewriteTransport rewrites every outgoing request to the mock server host,
// preserving path and query.
type rewriteTransport struct {
	base http.RoundTripper
	host string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := url.Parse(rt.host)
	if err != nil {
		return nil, err
	}
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = target.Scheme
	r2.URL.Host = target.Host
	return rt.base.RoundTrip(r2)
}
