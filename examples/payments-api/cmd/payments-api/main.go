package main

import (
	"fmt"
	"net/http"
	"os"
	"payments/internal/api"
	"payments/internal/metrics"
	"time"
)

func main() {
	// Because this is a POC, going to feed the app a mockup of dependencies and their health status
	// from a JSON file
	readyPath := getEnv("READINESS_FILE", "config/dependencies.json")

	mux := http.NewServeMux()
	metricsMux := http.NewServeMux()

	mux.HandleFunc("/", api.RootHandler)
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
		fmt.Println("Server listening on port 8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server Failed: %s\n", err)
		}
	}()

	go func() {
		fmt.Println("Server listening on port 9090...")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server failed: %s\n", err)
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
