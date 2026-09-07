package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration values for the Ethiyo service.
type Config struct {
	// Server settings
	Port string
	Env  string

	// Chalo API settings
	ChaloBaseURL   string
	RequestTimeout time.Duration

	// Browser automation settings
	BrowserPath           string
	BrowserHeadless       bool
	BrowserTimeout        time.Duration
	BrowserMaxConcurrency int

	// Google OAuth settings
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	OAuthStateSecret   string

	// Security and middleware settings
	CORSAllowedOrigins []string
	RateLimitRPS       float64
	RateLimitBurst     int
}

// Load reads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		Env:                   getEnv("ENV", "development"),
		ChaloBaseURL:          strings.TrimRight(getEnv("CHALO_BASE_URL", "https://chalo.com"), "/"),
		RequestTimeout:        getDurationEnv("REQUEST_TIMEOUT", 15*time.Second),
		BrowserPath:           getEnv("BROWSER_PATH", ""),
		BrowserHeadless:       getBoolEnv("BROWSER_HEADLESS", true),
		BrowserTimeout:        getDurationEnv("BROWSER_TIMEOUT", 15*time.Second),
		BrowserMaxConcurrency: getIntEnv("BROWSER_MAX_CONCURRENCY", 4),
		GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:     getEnv("GOOGLE_REDIRECT_URL", ""),
		OAuthStateSecret:      getEnv("OAUTH_STATE_SECRET", ""),
		RateLimitRPS:          getFloatEnv("RATE_LIMIT_RPS", 10.0),
		RateLimitBurst:        getIntEnv("RATE_LIMIT_BURST", 20),
	}

	origins := getEnv("CORS_ALLOWED_ORIGINS", "*")
	cfg.CORSAllowedOrigins = strings.Split(origins, ",")
	for i := range cfg.CORSAllowedOrigins {
		cfg.CORSAllowedOrigins[i] = strings.TrimSpace(cfg.CORSAllowedOrigins[i])
	}

	// Generate a secure random secret for OAuth state if not provided
	if cfg.OAuthStateSecret == "" {
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random oauth state secret: %w", err)
		}
		cfg.OAuthStateSecret = hex.EncodeToString(randomBytes)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks critical configuration fields.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT must not be empty")
	}
	if c.BrowserMaxConcurrency <= 0 {
		return fmt.Errorf("BROWSER_MAX_CONCURRENCY must be greater than 0")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("REQUEST_TIMEOUT must be positive")
	}
	if c.BrowserTimeout <= 0 {
		return fmt.Errorf("BROWSER_TIMEOUT must be positive")
	}
	if c.RateLimitRPS <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPS must be positive")
	}
	if c.RateLimitBurst <= 0 {
		return fmt.Errorf("RATE_LIMIT_BURST must be positive")
	}
	return nil
}

// IsOAuthEnabled returns true if Google OAuth credentials are fully provided.
func (c *Config) IsOAuthEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

func getEnv(key, defaultVal string) string {
	val, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(val) == "" {
		return defaultVal
	}
	return strings.TrimSpace(val)
}

func getBoolEnv(key string, defaultVal bool) bool {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}
	b, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return b
}

func getIntEnv(key string, defaultVal int) int {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return n
}

func getFloatEnv(key string, defaultVal float64) float64 {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}
	d, err := time.ParseDuration(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return d
}
