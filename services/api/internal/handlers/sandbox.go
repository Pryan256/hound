package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
	"github.com/hound-fi/api/internal/models"
)

// sandboxOnly returns a 403 if the request isn't coming from a test-env key.
func sandboxOnly(w http.ResponseWriter, r *http.Request) bool {
	env, _ := r.Context().Value(middleware.ContextKeyEnv).(string)
	if env != "test" {
		writeError(w, http.StatusForbidden, "SANDBOX_ONLY",
			"sandbox endpoints are only available for test-environment API keys")
		return false
	}
	return true
}

// SandboxFireWebhook manually triggers a webhook event for a sandbox item.
// Useful for testing webhook handler code without waiting for real events.
//
// POST /v1/sandbox/item/fire_webhook
// Body: { "access_token": "...", "webhook_type": "TRANSACTIONS_SYNC" }
func (h *Handler) SandboxFireWebhook(w http.ResponseWriter, r *http.Request) {
	if !sandboxOnly(w, r) {
		return
	}
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)

	var req struct {
		AccessToken string `json:"access_token"`
		WebhookType string `json:"webhook_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if req.AccessToken == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "access_token is required")
		return
	}
	validTypes := map[string]bool{
		models.EventTransactionsSync:  true,
		models.EventItemError:         true,
		models.EventItemLoginRequired: true,
	}
	if !validTypes[req.WebhookType] {
		writeError(w, http.StatusBadRequest, "INVALID_WEBHOOK_TYPE",
			"webhook_type must be one of: TRANSACTIONS_SYNC, ITEM_ERROR, ITEM_LOGIN_REQUIRED")
		return
	}

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, req.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	payload := map[string]any{
		"webhook_type": req.WebhookType,
		"item_id":      item.ID,
		"environment":  "sandbox",
	}

	go h.webhooks.Fire(r.Context(), appID, req.WebhookType, payload)

	writeJSON(w, http.StatusOK, map[string]any{
		"fired":        true,
		"webhook_type": req.WebhookType,
		"item_id":      item.ID,
		"request_id":   r.Header.Get("X-Request-ID"),
	})
}

// SandboxResetLogin puts a sandbox item into the LOGIN_REQUIRED error state and
// fires an ITEM_LOGIN_REQUIRED webhook, simulating an expired user session.
//
// POST /v1/sandbox/item/reset_login
// Body: { "access_token": "..." }
func (h *Handler) SandboxResetLogin(w http.ResponseWriter, r *http.Request) {
	if !sandboxOnly(w, r) {
		return
	}
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)

	var req struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if req.AccessToken == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "access_token is required")
		return
	}

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, req.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	if err := h.db.MarkItemError(r.Context(), item.ID, "sandbox_reset_login"); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update item")
		return
	}

	payload := map[string]any{
		"webhook_type": models.EventItemLoginRequired,
		"item_id":      item.ID,
		"error": map[string]string{
			"error_type":    "ITEM_ERROR",
			"error_code":    "ITEM_LOGIN_REQUIRED",
			"error_message": "the user's login credentials for this institution are no longer valid",
		},
		"environment": "sandbox",
	}

	go h.webhooks.Fire(r.Context(), appID, models.EventItemLoginRequired, payload)

	writeJSON(w, http.StatusOK, map[string]any{
		"reset":      true,
		"item_id":    item.ID,
		"new_status": models.ItemStatusError,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}
