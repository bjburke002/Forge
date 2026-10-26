package main

import (
	"log/slog"
	"net/http"
	"os"
	"payments/internal/api"
	"payments/internal/metrics"
	"strconv"
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

	finish := make(chan bool) // New goroutine channel

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
		}
	}()

	go func() {
		slog.Info("Server listening", "addr", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "addr", metricsServer.Addr, "error", err)
		}

	}()
	<-finish
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
