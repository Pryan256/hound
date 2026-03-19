package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/handlers"
	"github.com/hound-fi/api/internal/middleware"
	"github.com/hound-fi/api/internal/ratelimit"
	"github.com/hound-fi/api/internal/webhook"
	"go.uber.org/zap"
)

// New builds the HTTP router. The aggregator router and webhook dispatcher are
// constructed in main so they can be shared with background jobs.
func New(cfg *config.Config, db *database.DB, agg *aggregator.Router, webhooks *webhook.Dispatcher, limiter *ratelimit.Limiter, log *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(log))
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"https://*", "http://localhost:*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "Hound-Client-ID", "Hound-Access-Token"},
		MaxAge:         300,
	}))

	// Handlers
	h := handlers.New(db, agg, log, cfg, webhooks)

	// Health check (no auth)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Static assets — embed script (hound.js) and compiled Link widget
	// Served from ./static/ on disk (populated by the Docker build stage).
	staticFS := http.FileServer(http.Dir("./static"))
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		// Strip the /static prefix before delegating to the file server
		http.StripPrefix("/static", staticFS).ServeHTTP(w, r)
	})

	// Developer portal + management API (protected by ADMIN_SECRET)
	r.Get("/portal", h.Portal)
	r.Route("/management", func(r chi.Router) {
		r.Use(middleware.AdminAuth(cfg.AdminSecret, log))
		r.Get("/applications", h.ListApplications)
		r.Post("/applications", h.CreateApplication)
		r.Get("/applications/{appID}/keys", h.ListAPIKeys)
		r.Post("/applications/{appID}/keys", h.CreateAPIKey)
		r.Delete("/applications/{appID}/keys/{keyID}", h.RevokeAPIKey)
	})

	// Public browser-facing pages (no auth)
	r.Get("/demo", h.Demo)
	r.Get("/link/widget", h.LinkWidget)
	r.Get("/link/oauth/complete", h.OAuthComplete)

	// /link/ — browser-facing routes authenticated by link_token (called from Link widget)
	r.Route("/link", func(r chi.Router) {
		r.Use(middleware.LinkTokenAuth(db, log))

		// Institution search (called as user types in Link widget)
		r.Get("/institutions/search", h.SearchInstitutions)
		r.Get("/institutions", h.GetInstitution)

		// OAuth flow
		r.Post("/oauth/initiate", h.OAuthInitiate)
		r.Post("/oauth/callback", h.OAuthCallback)
	})

	// v1 API — all routes require API key auth (server-side only)
	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(db, log))
		if limiter != nil {
			r.Use(middleware.RateLimit(limiter, log))
		}

		// Link token (initiates a Link session)
		r.Post("/link/token/create", h.CreateLinkToken)

		// Exchange public token for access token after Link completes
		r.Post("/item/public_token/exchange", h.ExchangePublicToken)

		// Item management
		r.Get("/item", h.GetItem)
		r.Delete("/item", h.DeleteItem)

		// Financial data
		r.Get("/accounts", h.GetAccounts)
		r.Get("/accounts/balance", h.GetAccountBalance)
		r.Get("/transactions", h.GetTransactions)
		r.Get("/identity", h.GetIdentity)

		// Webhooks
		r.Post("/webhooks", h.RegisterWebhook)
		r.Get("/webhooks", h.ListWebhooks)
		r.Delete("/webhooks/{webhookID}", h.DeleteWebhook)

		// Sandbox control (test keys only — handlers enforce env=test)
		r.Post("/sandbox/item/fire_webhook", h.SandboxFireWebhook)
		r.Post("/sandbox/item/reset_login", h.SandboxResetLogin)
	})

	return r
}
