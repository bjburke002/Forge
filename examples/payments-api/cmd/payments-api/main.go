package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"payments/internal/api"
	"payments/internal/metrics"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// Setting slog default to json for AWS logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Because this is a POC, going to feed the app a mockup of dependencies and their health status
	// from a JSON file
	readyPath := getEnv("READINESS_FILE", "config/dependencies.json")

	// ERROR_RATE is used to simulate 5xx returns. Num must be between 0-1
	errorRate, err := getFloatEnv("ERROR_RATE", 0)
	if err != nil {
		slog.Error("Parsing ERROR_RATE env", "value", os.Getenv("ERROR_RATE"), "error", err)
		os.Exit(1)
	}

	if errorRate < 0 || errorRate > 1 {
		slog.Error("Validating ERROR_RATE", "value", errorRate, "error", "must be between 0 and 1")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	metricsMux := http.NewServeMux()

	mux.HandleFunc("/", api.RootHandler(errorRate))
	mux.HandleFunc("/health/live", api.LivenessHandler)
	mux.HandleFunc("/health/ready", api.ReadinessHandler(readyPath))
	metricsMux.Handle("/metrics", metrics.Handler())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr:         ":8080",
		Handler:      metrics.Middleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Using port 9090 for metrics to keep requests isolated
	metricsServer := &http.Server{
		Addr:         ":9090",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Goroutine will run both of these HTTP servers in parallel
	go func() {
		slog.Info("Server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "addr", server.Addr, "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		slog.Info("Server listening", "addr", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "addr", metricsServer.Addr, "error", err)
			os.Exit(1)
		}

	}()
	<-ctx.Done()
	slog.Info("Shutdown signal received, stopping servers")

	// Hard timeout context for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt to gracefully shutdown the server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "addr", server.Addr, "error", err)
	} else {
		slog.Info("Server stopped", "addr", server.Addr)
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "addr", metricsServer.Addr, "error", err)
	} else {
		slog.Info("Server stopped", "addr", metricsServer.Addr)
	}
}

// Rolling my own getEnv to set a fallback default value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Helper to pull ERROR_RATE environment variable
func getFloatEnv(key string, fallback float64) (float64, error) {
	if value, exists := os.LookupEnv(key); exists {
		return strconv.ParseFloat(value, 64)
	}
	return fallback, nil
}
