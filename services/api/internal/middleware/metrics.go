package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hound-fi/api/internal/metrics"
)

// Metrics is a Chi middleware that records per-route request counts and latency.
//
// The route label uses the matched route pattern (e.g. /v1/webhooks/{webhookID})
// rather than the raw URL path, preventing label cardinality explosion from
// path parameters. The pattern is read after next.ServeHTTP returns because
// chi finalises the full pattern only after descending into nested routers.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			// Skip recording metrics for the /metrics endpoint itself.
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unknown"
			}
			if route == "/metrics" {
				return
			}

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.status)

			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
		})
	}
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.wroteHeader {
		sr.status = code
		sr.wroteHeader = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

// Write captures a 200 if WriteHeader was never called explicitly.
func (sr *statusRecorder) Write(b []byte) (int, error) {
	if !sr.wroteHeader {
		sr.status = http.StatusOK
		sr.wroteHeader = true
	}
	return sr.ResponseWriter.Write(b)
}
