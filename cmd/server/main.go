package main

import (
	"log/slog"
	"os"

	"github.com/R-zin/Ethiyo/internal/config"
	"github.com/R-zin/Ethiyo/internal/server"
)

func main() {
	// Initialize structured logger
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("initializing Ethiyo server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create server instance
	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("failed to initialize server", "error", err)
		os.Exit(1)
	}

	// Start server with graceful shutdown
	if err := srv.Start(); err != nil {
		slog.Error("server encountered fatal error", "error", err)
		os.Exit(1)
	}
}
