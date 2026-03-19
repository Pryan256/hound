package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/ratelimit"
	"go.uber.org/zap"
)

// RateLimit is a Chi middleware that enforces per-API-key rate limits.
// Must run after APIKeyAuth so ContextKeyAPIKeyID and ContextKeyEnv are populated.
func RateLimit(limiter *ratelimit.Limiter, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID, ok := r.Context().Value(ContextKeyAPIKeyID).(uuid.UUID)
			if !ok {
				// APIKeyAuth didn't run — pass through (health check, etc.)
				next.ServeHTTP(w, r)
				return
			}
			env, _ := r.Context().Value(ContextKeyEnv).(string)

			res := limiter.Allow(r.Context(), keyID, env)

			// Always set headers so developers can monitor their usage.
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetAt.Unix(), 10))

			if !res.Allowed {
				retryAfter := int(time.Until(res.ResetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				log.Warn("rate limit exceeded",
					zap.String("key_id", keyID.String()),
					zap.String("env", env),
					zap.String("path", r.URL.Path),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w,
					`{"error_code":"RATE_LIMIT_EXCEEDED","error_message":"rate limit of %d req/min exceeded — retry after %d seconds"}`,
					res.Limit, retryAfter,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
