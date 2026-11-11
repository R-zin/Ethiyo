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

func TestCORSOriginsParsedAndTrimmed(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com ,https://c.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com"}
	if len(cfg.CORSAllowedOrigins) != len(want) {
		t.Fatalf("expected %d origins, got %v", len(want), cfg.CORSAllowedOrigins)
	}
	for i, o := range want {
		if cfg.CORSAllowedOrigins[i] != o {
			t.Errorf("origin %d: expected %q, got %q", i, o, cfg.CORSAllowedOrigins[i])
		}
	}
}

func TestOAuthStateSecretRandomAcrossLoads(t *testing.T) {
	_ = os.Unsetenv("OAUTH_STATE_SECRET")
	c1, err := Load()
	if err != nil {
		t.Fatalf("Load 1: %v", err)
	}
	c2, err := Load()
	if err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	if c1.OAuthStateSecret == c2.OAuthStateSecret {
		t.Errorf("expected distinct random secrets across Load calls")
	}
}

func TestInvalidEnvValuesFallBackToDefaults(t *testing.T) {
	t.Setenv("PORT", "   ") // blank → default
	t.Setenv("BROWSER_MAX_CONCURRENCY", "notanumber")
	t.Setenv("RATE_LIMIT_RPS", "abc")
	t.Setenv("REQUEST_TIMEOUT", "not-a-duration")
	t.Setenv("BROWSER_HEADLESS", "maybe")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("blank PORT should fall back to 8080, got %q", cfg.Port)
	}
	if cfg.BrowserMaxConcurrency != 4 {
		t.Errorf("bad int should fall back to 4, got %d", cfg.BrowserMaxConcurrency)
	}
	if cfg.RateLimitRPS != 10.0 {
		t.Errorf("bad float should fall back to 10, got %v", cfg.RateLimitRPS)
	}
	if cfg.RequestTimeout != 15*time.Second {
		t.Errorf("bad duration should fall back to 15s, got %v", cfg.RequestTimeout)
	}
	if !cfg.BrowserHeadless {
		t.Errorf("bad bool should fall back to true")
	}
}

func TestTrustedProxiesAndScraperDefaults(t *testing.T) {
	_ = os.Unsetenv("TRUSTED_PROXIES")
	_ = os.Unsetenv("SCRAPER_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.ScraperEnabled {
		t.Errorf("expected ScraperEnabled default true, got false")
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("expected default TrustedProxies to be empty, got %v", cfg.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.0.0.2, 192.168.1.1/24 ")
	t.Setenv("SCRAPER_ENABLED", "false")

	cfgCustom, err := Load()
	if err != nil {
		t.Fatalf("Load with custom: %v", err)
	}
	if cfgCustom.ScraperEnabled {
		t.Errorf("expected ScraperEnabled false, got true")
	}
	expectedProxies := []string{"10.0.0.1", "10.0.0.2", "192.168.1.1/24"}
	if len(cfgCustom.TrustedProxies) != len(expectedProxies) {
		t.Fatalf("expected %d trusted proxies, got %v", len(expectedProxies), cfgCustom.TrustedProxies)
	}
	for i, want := range expectedProxies {
		if cfgCustom.TrustedProxies[i] != want {
			t.Errorf("proxy %d: expected %q, got %q", i, want, cfgCustom.TrustedProxies[i])
		}
	}
}

func TestProductionValidation(t *testing.T) {
	baseCfg := func() *Config {
		return &Config{
			Port:                  "8080",
			Env:                   "production",
			BrowserMaxConcurrency: 4,
			RequestTimeout:        10 * time.Second,
			BrowserTimeout:        10 * time.Second,
			RateLimitRPS:          10,
			RateLimitBurst:        20,
			CORSAllowedOrigins:    []string{"https://example.com"},
		}
	}

	// Case 1: Wildcard CORS rejected in production
	cfgWildcard := baseCfg()
	cfgWildcard.CORSAllowedOrigins = []string{"*"}
	if err := cfgWildcard.Validate(); err == nil {
		t.Errorf("expected error for wildcard CORS origin in production")
	}

	// Case 2: Specific origin accepted in production
	cfgSpecific := baseCfg()
	if err := cfgSpecific.Validate(); err != nil {
		t.Errorf("expected specific origin to pass in production, got: %v", err)
	}

	// Case 3: OAuth enabled in production without explicit OAUTH_STATE_SECRET env
	cfgOAuth := baseCfg()
	cfgOAuth.GoogleClientID = "client-id"
	cfgOAuth.GoogleClientSecret = "secret"
	cfgOAuth.GoogleRedirectURL = "https://example.com/callback"

	_ = os.Unsetenv("OAUTH_STATE_SECRET")
	if err := cfgOAuth.Validate(); err == nil {
		t.Errorf("expected error when OAuth enabled in production without explicit OAUTH_STATE_SECRET")
	}

	// Case 4: OAuth enabled in production with explicit OAUTH_STATE_SECRET
	t.Setenv("OAUTH_STATE_SECRET", "explicit-production-secret")
	if err := cfgOAuth.Validate(); err != nil {
		t.Errorf("expected OAuth with explicit secret to pass validation, got: %v", err)
	}
}
