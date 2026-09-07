package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars
	keys := []string{
		"PORT", "ENV", "CHALO_BASE_URL", "REQUEST_TIMEOUT",
		"BROWSER_PATH", "BROWSER_HEADLESS", "BROWSER_TIMEOUT",
		"BROWSER_MAX_CONCURRENCY", "GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
		"OAUTH_STATE_SECRET", "CORS_ALLOWED_ORIGINS",
		"RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading defaults: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got %q", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("expected default Env 'development', got %q", cfg.Env)
	}
	if cfg.ChaloBaseURL != "https://chalo.com" {
		t.Errorf("expected default ChaloBaseURL 'https://chalo.com', got %q", cfg.ChaloBaseURL)
	}
	if cfg.RequestTimeout != 15*time.Second {
		t.Errorf("expected default RequestTimeout 15s, got %v", cfg.RequestTimeout)
	}
	if !cfg.BrowserHeadless {
		t.Errorf("expected default BrowserHeadless true, got false")
	}
	if cfg.BrowserMaxConcurrency != 4 {
		t.Errorf("expected default BrowserMaxConcurrency 4, got %d", cfg.BrowserMaxConcurrency)
	}
	if cfg.OAuthStateSecret == "" {
		t.Errorf("expected auto-generated OAuthStateSecret, got empty")
	}
	if cfg.IsOAuthEnabled() {
		t.Errorf("expected IsOAuthEnabled to be false when env is not set")
	}
}

func TestLoadWithCustomEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("CHALO_BASE_URL", "https://custom.chalo.com/")
	t.Setenv("BROWSER_HEADLESS", "false")
	t.Setenv("BROWSER_MAX_CONCURRENCY", "8")
	t.Setenv("REQUEST_TIMEOUT", "30s")
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost:9090/callback")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected Port '9090', got %q", cfg.Port)
	}
	if cfg.ChaloBaseURL != "https://custom.chalo.com" {
		t.Errorf("expected trimmed ChaloBaseURL, got %q", cfg.ChaloBaseURL)
	}
	if cfg.BrowserHeadless {
		t.Errorf("expected BrowserHeadless false, got true")
	}
	if cfg.BrowserMaxConcurrency != 8 {
		t.Errorf("expected BrowserMaxConcurrency 8, got %d", cfg.BrowserMaxConcurrency)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("expected RequestTimeout 30s, got %v", cfg.RequestTimeout)
	}
	if !cfg.IsOAuthEnabled() {
		t.Errorf("expected IsOAuthEnabled true, got false")
	}
}

func TestValidationErrors(t *testing.T) {
	cfg := &Config{
		Port:                  "",
		BrowserMaxConcurrency: 4,
		RequestTimeout:        10 * time.Second,
		BrowserTimeout:        10 * time.Second,
		RateLimitRPS:          10,
		RateLimitBurst:        20,
	}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on empty Port")
	}

	cfg.Port = "8080"
	cfg.BrowserMaxConcurrency = 0
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error on BrowserMaxConcurrency <= 0")
	}
}
