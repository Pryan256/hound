package middleware

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// AdminAuth protects management endpoints with a static ADMIN_SECRET token.
// Pass the secret as: Authorization: Bearer <ADMIN_SECRET>
// This is intentionally simple for the MVP — not per-user auth.
func AdminAuth(secret string, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no secret is configured (dev mode), allow all requests.
			if secret == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != secret {
				log.Warn("admin auth failed", zap.String("path", r.URL.Path))
				writeUnauthorized(w, "invalid admin secret")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
