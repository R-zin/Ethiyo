package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/R-zin/Ethiyo/internal/auth"
	"github.com/R-zin/Ethiyo/internal/browser"
	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/config"
	"github.com/R-zin/Ethiyo/internal/handlers"
	"github.com/R-zin/Ethiyo/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Server encapsulates the HTTP server and its dependencies.
type Server struct {
	cfg         *config.Config
	engine      *gin.Engine
	http        *http.Server
	rateLimiter *middleware.RateLimiter
	browserMgr  *browser.Manager
	running     atomic.Bool
}

// New constructs a configured Server instance.
func New(cfg *config.Config) (*Server, error) {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 1. Locate browser executable
	browserPath, err := browser.FindBrowserExecutable(cfg.BrowserPath)
	if err != nil {
		slog.Warn("browser executable not found at startup; browser-dependent endpoints may fail", "error", err)
	} else {
		slog.Info("browser executable located", "path", browserPath)
	}

	// 2. Initialize dependencies
	browserMgr := browser.NewManager(
		browserPath,
		cfg.BrowserHeadless,
		cfg.BrowserTimeout,
		cfg.BrowserMaxConcurrency,
	)

	chaloClient := chalo.NewClient(cfg.ChaloBaseURL, cfg.RequestTimeout)
	chaloScraper := chalo.NewScraper(browserMgr, cfg.ChaloBaseURL, cfg.BrowserTimeout)
	chaloService := chalo.NewService(
		chaloClient,
		chaloScraper,
		chalo.WithScraperEnabled(cfg.ScraperEnabled),
	)

	oauthService := auth.NewGoogleService(cfg)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	srv := &Server{
		cfg:         cfg,
		rateLimiter: rateLimiter,
		browserMgr:  browserMgr,
	}
	srv.running.Store(true)

	// 3. Initialize HTTP handlers
	healthHandler := handlers.NewHealthHandler(browserPath, srv.running.Load)
	busHandler := handlers.NewBusHandler(chaloService)
	isSecureCookie := cfg.Env == "production"
	authHandler := handlers.NewAuthHandler(oauthService, isSecureCookie)

	// 4. Setup Gin engine with middleware
	engine := gin.New()
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("failed to configure trusted proxies: %w", err)
	}

	engine.Use(
		middleware.RequestID(),
		middleware.Logger(),
		middleware.Recovery(),
		middleware.SecurityHeaders(),
		middleware.RequestSizeLimit(2*1024*1024), // 2MB
		middleware.CORS(cfg.CORSAllowedOrigins),
		rateLimiter.Middleware(),
	)

	// 5. Register Routes
	// Health and readiness routes
	engine.GET("/health", healthHandler.Check)
	engine.GET("/api/v1/health", healthHandler.Check)
	engine.GET("/ready", healthHandler.Ready)
	engine.GET("/api/v1/ready", healthHandler.Ready)

	// Modern V1 routes
	v1 := engine.Group("/api/v1")
	{
		busGroup := v1.Group("/bus")
		{
			busGroup.GET("/:buscode/track", busHandler.TrackBus)
			busGroup.GET("/:buscode/route", busHandler.RouteDetails)
			busGroup.GET("/:buscode/url", busHandler.GetURL)
		}

		authGroup := v1.Group("/auth/google")
		{
			authGroup.GET("/login", authHandler.Login)
			authGroup.GET("/callback", authHandler.Callback)
		}
	}

	// Legacy backward-compatible routes
	engine.GET("/trackroute", busHandler.LegacyTrackRoute)
	engine.GET("/trackxhr", busHandler.LegacyTrackXHR)
	engine.GET("/testroute", busHandler.LegacyTestRoute)
	engine.GET("/get_bus_url", busHandler.LegacyGetBusURL)
	engine.GET("/auth/google/login", authHandler.Login)
	engine.GET("/auth/google/callback", authHandler.Callback)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.RequestTimeout + 5*time.Second,
		WriteTimeout:      cfg.BrowserTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srv.engine = engine
	srv.http = httpServer

	return srv, nil
}

// Engine returns the underlying Gin engine (useful for testing).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// IsRunning returns whether the server is currently marked running.
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// Close gracefully closes the server and frees resources.
func (s *Server) Close() error {
	s.running.Store(false)
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	if s.browserMgr != nil {
		_ = s.browserMgr.Close()
	}
	if s.http != nil {
		return s.http.Close()
	}
	return nil
}

// Start runs the HTTP server and blocks until an interrupt signal is received.
func (s *Server) Start() error {
	serverErr := make(chan error, 1)

	go func() {
		slog.Info("server listening", "port", s.cfg.Port, "env", s.cfg.Env)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("shutting down server...", "signal", sig.String())
	}

	s.running.Store(false)
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	if s.browserMgr != nil {
		_ = s.browserMgr.Close()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced shutdown: %w", err)
	}

	slog.Info("server exited cleanly")
	return nil
}
