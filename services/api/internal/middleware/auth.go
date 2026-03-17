package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/hound-fi/api/internal/database"
	"go.uber.org/zap"
)

type contextKey string

const (
	ContextKeyApplicationID contextKey = "application_id"
	ContextKeyAPIKeyID      contextKey = "api_key_id"
	ContextKeyEnv           contextKey = "key_env" // "test" | "live"
)

// APIKeyAuth validates the Authorization: Bearer <key> header.
// Keys are prefixed: "hound_test_" or "hound_live_"
func APIKeyAuth(db *database.DB, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeUnauthorized(w, "invalid authorization format")
				return
			}

			rawKey := parts[1]
			if !strings.HasPrefix(rawKey, "hound_test_") && !strings.HasPrefix(rawKey, "hound_live_") {
				writeUnauthorized(w, "invalid api key format")
				return
			}

			apiKey, err := db.ValidateAPIKey(r.Context(), rawKey)
			if err != nil {
				log.Warn("api key validation failed", zap.Error(err))
				writeUnauthorized(w, "invalid or revoked api key")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyApplicationID, apiKey.ApplicationID)
			ctx = context.WithValue(ctx, ContextKeyAPIKeyID, apiKey.ID)
			ctx = context.WithValue(ctx, ContextKeyEnv, apiKey.Env)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error_code":"UNAUTHORIZED","error_message":"` + msg + `"}`))
}
