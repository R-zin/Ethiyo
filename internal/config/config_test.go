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

func TestLoadNewFeatureDefaultsAndOverrides(t *testing.T) {
	for _, k := range []string{"LIVE_CACHE_TTL", "ROUTE_CACHE_TTL", "STREAM_POLL_INTERVAL",
		"STREAM_MAX_CLIENTS", "STREAM_HEARTBEAT", "STREAM_MAX_LIFETIME", "METRICS_ENABLED"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LiveCacheTTL != 5*time.Second || cfg.RouteCacheTTL != 10*time.Minute {
		t.Errorf("cache TTL defaults = %v/%v", cfg.LiveCacheTTL, cfg.RouteCacheTTL)
	}
	if cfg.StreamPollInterval != 10*time.Second || cfg.StreamMaxClients != 500 ||
		cfg.StreamHeartbeat != 15*time.Second || cfg.StreamMaxLifetime != 30*time.Minute {
		t.Errorf("stream defaults = %v/%d/%v/%v", cfg.StreamPollInterval, cfg.StreamMaxClients, cfg.StreamHeartbeat, cfg.StreamMaxLifetime)
	}
	if !cfg.MetricsEnabled {
		t.Error("metrics should be enabled by default")
	}

	t.Setenv("LIVE_CACHE_TTL", "0s") // explicit disable is valid
	t.Setenv("STREAM_MAX_CLIENTS", "42")
	t.Setenv("METRICS_ENABLED", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load overrides: %v", err)
	}
	if cfg.LiveCacheTTL != 0 || cfg.StreamMaxClients != 42 || cfg.MetricsEnabled {
		t.Errorf("overrides not applied: ttl=%v max=%d metrics=%v", cfg.LiveCacheTTL, cfg.StreamMaxClients, cfg.MetricsEnabled)
	}
}

func TestValidateRejectsInvalidFeatureSettings(t *testing.T) {
	base := func() *Config {
		return &Config{Port: "8080", BrowserMaxConcurrency: 1, RequestTimeout: time.Second,
			BrowserTimeout: time.Second, RateLimitRPS: 1, RateLimitBurst: 1,
			StreamPollInterval: time.Second, StreamMaxClients: 1,
			StreamHeartbeat: time.Second, StreamMaxLifetime: time.Minute,
			BreakerThreshold: 5, BreakerCooldown: time.Second}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid base config rejected: %v", err)
	}
	mutations := map[string]func(*Config){
		"negative live ttl":   func(c *Config) { c.LiveCacheTTL = -time.Second },
		"negative route ttl":  func(c *Config) { c.RouteCacheTTL = -time.Second },
		"zero poll interval":  func(c *Config) { c.StreamPollInterval = 0 },
		"zero max clients":    func(c *Config) { c.StreamMaxClients = 0 },
		"zero heartbeat":      func(c *Config) { c.StreamHeartbeat = 0 },
		"zero lifetime":       func(c *Config) { c.StreamMaxLifetime = 0 },
		"zero breaker limit":  func(c *Config) { c.BreakerThreshold = 0 },
		"zero cooldown":       func(c *Config) { c.BreakerCooldown = 0 },
		"negative last-known": func(c *Config) { c.LastKnownTTL = -time.Second },
	}
	for name, mutate := range mutations {
		c := base()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestLoadResilienceSettings(t *testing.T) {
	for _, k := range []string{"UPSTREAM_BREAKER_THRESHOLD", "UPSTREAM_BREAKER_COOLDOWN", "LAST_KNOWN_TTL"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BreakerThreshold != 5 || cfg.BreakerCooldown != 30*time.Second || cfg.LastKnownTTL != 15*time.Minute {
		t.Errorf("defaults = %d/%v/%v", cfg.BreakerThreshold, cfg.BreakerCooldown, cfg.LastKnownTTL)
	}

	t.Setenv("UPSTREAM_BREAKER_THRESHOLD", "3")
	t.Setenv("UPSTREAM_BREAKER_COOLDOWN", "5s")
	t.Setenv("LAST_KNOWN_TTL", "0s")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load overrides: %v", err)
	}
	if cfg.BreakerThreshold != 3 || cfg.BreakerCooldown != 5*time.Second || cfg.LastKnownTTL != 0 {
		t.Errorf("overrides = %d/%v/%v", cfg.BreakerThreshold, cfg.BreakerCooldown, cfg.LastKnownTTL)
	}

	t.Setenv("UPSTREAM_BREAKER_THRESHOLD", "0")
	if _, err := Load(); err == nil {
		t.Error("a zero breaker threshold must be rejected")
	}
}
