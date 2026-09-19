package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/config"
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
}
