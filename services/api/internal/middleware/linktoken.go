package middleware

import (
	"context"
	"net/http"

	"github.com/hound-fi/api/internal/database"
	"go.uber.org/zap"
)

const (
	ContextKeyLinkEnv    contextKey = "link_env"
	ContextKeyLinkUserID contextKey = "link_user_id"
)

// LinkTokenAuth validates the link_token query parameter.
// Used for browser-facing endpoints called by the Link widget (not the developer's server).
// Link tokens are short-lived (30 min) and scoped to a single Link session.
func LinkTokenAuth(db *database.DB, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("link_token")
			if token == "" {
				writeUnauthorized(w, "missing link_token")
				return
			}

			session, err := db.ValidateLinkToken(r.Context(), token)
			if err != nil {
				log.Warn("link token validation failed", zap.Error(err))
				writeUnauthorized(w, "invalid or expired link_token")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyApplicationID, session.ApplicationID)
			ctx = context.WithValue(ctx, ContextKeyLinkEnv, session.Env)
			ctx = context.WithValue(ctx, ContextKeyLinkUserID, session.UserID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
