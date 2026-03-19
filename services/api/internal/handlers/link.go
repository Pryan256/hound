package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
	"github.com/hound-fi/api/internal/models"
	"go.uber.org/zap"
)

type CreateLinkTokenRequest struct {
	UserID      string   `json:"user_id"`       // your app's user identifier
	Products    []string `json:"products"`      // ["transactions", "identity"]
	CountryCodes []string `json:"country_codes"` // ["US"]
	RedirectURI string   `json:"redirect_uri"`
}

type CreateLinkTokenResponse struct {
	LinkToken  string    `json:"link_token"`
	Expiration time.Time `json:"expiration"`
	RequestID  string    `json:"request_id"`
}

type ExchangePublicTokenRequest struct {
	PublicToken string `json:"public_token"`
}

type ExchangePublicTokenResponse struct {
	AccessToken string    `json:"access_token"`
	ItemID      uuid.UUID `json:"item_id"`
	RequestID   string    `json:"request_id"`
}

func (h *Handler) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	env, _ := r.Context().Value(middleware.ContextKeyEnv).(string)

	var req CreateLinkTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_id is required")
		return
	}

	token, expiry, err := h.db.CreateLinkToken(r.Context(), appID, req.UserID, req.Products, env)
	if err != nil {
		h.log.Error("failed to create link token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create link token")
		return
	}

	writeJSON(w, http.StatusOK, CreateLinkTokenResponse{
		LinkToken:  token,
		Expiration: expiry,
		RequestID:  r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) ExchangePublicToken(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)

	var req ExchangePublicTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	item, accessToken, err := h.db.ExchangePublicToken(r.Context(), appID, req.PublicToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PUBLIC_TOKEN", "public token is invalid or expired")
		return
	}

	writeJSON(w, http.StatusOK, ExchangePublicTokenResponse{
		AccessToken: accessToken,
		ItemID:      item.ID,
		RequestID:   r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"item":       item,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	// Revoke at the provider level
	if err := h.agg.RevokeItem(r.Context(), item); err != nil {
		h.log.Warn("failed to revoke item at provider", zap.String("item_id", item.ID.String()), zap.Error(err))
		// Continue with local deletion even if provider revocation fails
	}

	if err := h.db.DeleteItem(r.Context(), item.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove item")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":    true,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}

// CreateRelinkToken issues a Link token scoped to an existing errored item.
// The developer opens the Link widget with this token; when the user completes
// re-authentication the item's provider tokens are replaced in place and its
// status returns to active. All accounts and transaction history are preserved.
//
// POST /v1/item/relink/token
// Body: { "access_token": "...", "user_id": "..." }
func (h *Handler) CreateRelinkToken(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	env, _ := r.Context().Value(middleware.ContextKeyEnv).(string)

	var req struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if req.AccessToken == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "access_token is required")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_id is required")
		return
	}

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, req.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	if item.Status != models.ItemStatusError {
		writeError(w, http.StatusBadRequest, "ITEM_NOT_IN_ERROR",
			"item must be in error state to relink (current status: "+string(item.Status)+")")
		return
	}

	token, expiry, err := h.db.CreateRelinkToken(r.Context(), appID, req.UserID, item.ID, env)
	if err != nil {
		h.log.Error("failed to create relink token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create relink token")
		return
	}

	writeJSON(w, http.StatusOK, CreateLinkTokenResponse{
		LinkToken:  token,
		Expiration: expiry,
		RequestID:  r.Header.Get("X-Request-ID"),
	})
}

// Ensure models import is used
var _ = models.ItemStatusActive
