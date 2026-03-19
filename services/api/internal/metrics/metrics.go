// Package metrics defines and registers all Prometheus metrics for the Hound API.
//
// Import this package for its side effects — metrics are registered in init()
// and available globally via the exported variables.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTPRequestsTotal counts every completed HTTP request.
	// Labels: method (GET/POST/…), route (/v1/accounts), status (200/429/…).
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hound_http_requests_total",
			Help: "Total number of HTTP requests by method, route, and status code.",
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestDuration measures end-to-end request latency.
	// Labels: method, route.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hound_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "route"},
	)

	// RateLimitHitsTotal counts 429 responses issued by the rate limiter.
	// Labels: env (test/live).
	RateLimitHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hound_ratelimit_hits_total",
			Help: "Total number of requests rejected by the rate limiter.",
		},
		[]string{"env"},
	)

	// WebhookDeliveriesTotal counts outbound webhook delivery attempts.
	// Labels: outcome (success/retry/perm_failed).
	WebhookDeliveriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hound_webhook_deliveries_total",
			Help: "Total webhook delivery attempts by outcome.",
		},
		[]string{"outcome"},
	)

	// WebhookDeliveryDuration measures the round-trip time of outbound webhook POSTs.
	WebhookDeliveryDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "hound_webhook_delivery_duration_seconds",
			Help:    "Outbound webhook HTTP request duration in seconds.",
			Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	// TokenRefreshesTotal counts background token refresh attempts.
	// Labels: result (success/error/unsupported).
	TokenRefreshesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hound_token_refreshes_total",
			Help: "Background token refresh attempts by result.",
		},
		[]string{"result"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		RateLimitHitsTotal,
		WebhookDeliveriesTotal,
		WebhookDeliveryDuration,
		TokenRefreshesTotal,
	)
}
