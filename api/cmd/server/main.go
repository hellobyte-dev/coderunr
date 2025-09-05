package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coderunr/api/internal/config"
	"github.com/coderunr/api/internal/handler"
	"github.com/coderunr/api/internal/job"
	"github.com/coderunr/api/internal/middleware"
	"github.com/coderunr/api/internal/runtime"
	"github.com/coderunr/api/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Set up logging
	logger := logrus.New()
	logger.SetLevel(cfg.GetLogLevel())
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger.Info("Starting CodeRunr API Server")

	// Ensure data directories exist
	if err := ensureDataDirectories(cfg); err != nil {
		logger.WithError(err).Fatal("Failed to create data directories")
	}

	// Initialize runtime manager and load packages
	runtimeManager := runtime.NewManager(cfg)
	if err := runtimeManager.LoadPackages(); err != nil {
		logger.WithError(err).Fatal("Failed to load packages")
	}

	// Initialize job manager
	jobManager := job.NewManager(cfg)

	// Initialize package service
	packageService := service.NewPackageService(cfg, logger, runtimeManager)

	// Initialize handlers
	h := handler.NewHandler(jobManager, runtimeManager, logger)
	packageHandler := handler.NewPackageHandler(packageService, logger)

	// Set up router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.CORS())
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// API routes
	r.Route("/api/v2", func(r chi.Router) {
		// Apply JSON middleware to non-GET routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.JSON)
			r.Post("/execute", h.ExecuteCode)
		})

		// WebSocket route (no JSON middleware)
		r.HandleFunc("/connect", h.HandleWebSocket)

		// GET routes
		r.Get("/runtimes", h.GetRuntimes)

		// Package management routes
		packageHandler.RegisterRoutes(r)
	})

	// Root route
	r.Get("/", h.GetVersion)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create HTTP server
	server := &http.Server{
		Addr:    cfg.GetBindAddress(),
		Handler: r,
		// Security settings
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("API server starting on %s", cfg.GetBindAddress())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
		os.Exit(1)
	}

	logger.Info("Server exited")
}

// ensureDataDirectories ensures that all required data directories exist
func ensureDataDirectories(cfg *config.Config) error {
	directories := []string{
		cfg.DataDirectory,
		cfg.DataDirectory + "/packages",
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
