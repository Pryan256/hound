package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
)

// RegisterWebhook registers a new webhook endpoint for an application.
// POST /v1/webhooks
// Body: { "url": "https://...", "events": ["TRANSACTIONS_SYNC", "ITEM_ERROR"] }
func (h *Handler) RegisterWebhook(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)

	var req struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "http://") {
		writeError(w, http.StatusBadRequest, "INVALID_URL", "url must start with https:// (or http:// for local testing)")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "events must be a non-empty list")
		return
	}

	// Generate a signing secret — shown once, then only stored encrypted.
	rawSecret := webhookSecret()
	encSecret, err := h.enc.Encrypt(rawSecret)
	if err != nil {
		h.log.Sugar().Errorf("encrypt webhook secret: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate signing secret")
		return
	}

	webhook, err := h.db.CreateWebhook(r.Context(), appID, req.URL, req.Events, encSecret)
	if err != nil {
		h.log.Sugar().Errorf("create webhook: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to register webhook")
		return
	}

	// Attach the raw secret — this is the only time it will ever be returned.
	webhook.Secret = rawSecret

	writeJSON(w, http.StatusCreated, webhook)
}

// ListWebhooks returns all registered webhooks for an application.
// GET /v1/webhooks
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)

	webhooks, err := h.db.ListWebhooks(r.Context(), appID)
	if err != nil {
		h.log.Sugar().Errorf("list webhooks: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list webhooks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": webhooks})
}

func webhookSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// DeleteWebhook removes a registered webhook.
// DELETE /v1/webhooks/{webhookID}
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	webhookID, err := uuid.Parse(chi.URLParam(r, "webhookID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_WEBHOOK_ID", "invalid webhook id")
		return
	}

	if err := h.db.DeleteWebhook(r.Context(), webhookID, appID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
