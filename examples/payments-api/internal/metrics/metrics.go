package metrics

import (
	"net/http"

	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// httpRequestsTotal counts requests served on the main port, by method, matched
// route pattern and status code. promauto registers it with the default registry.
var httpRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, route and status code.",
	},
	[]string{"method", "route", "code"},
)

// httpRequestDuration generates a histogram for request duration in seconds
var httpRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "route"},
)

// Handler serves everything that Prom will scrape
func Handler() http.Handler {
	return promhttp.Handler()
}

// Custom responseWriter to grab status codes
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the original ResponseWriter so that http.ResponseController
// can reach features that the wrapper would hide
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Middlware creates a wrapper to capture information for Prometheus metrics scrapes
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		skip := strings.HasPrefix(r.URL.Path, "/health/") // Skip requests sent to the health endpoints

		if skip {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()

		// Wrap original response writer to intercept status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		// Record metrics after the request
		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(rw.statusCode)
		path := r.Pattern

		httpRequestsTotal.WithLabelValues(r.Method, path, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}
