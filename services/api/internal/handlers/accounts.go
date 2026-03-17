package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
	"go.uber.org/zap"
)

func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	if err := h.decryptItem(item); err != nil {
		h.log.Error("failed to decrypt item token", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token decryption failed")
		return
	}

	accounts, err := h.agg.GetAccounts(r.Context(), item)
	if err != nil {
		h.log.Error("failed to fetch accounts", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to fetch accounts from institution")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":   accounts,
		"item":       item,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	if err := h.decryptItem(item); err != nil {
		h.log.Error("failed to decrypt item token", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token decryption failed")
		return
	}

	// Real-time balance fetch (bypasses cache)
	accounts, err := h.agg.GetAccountBalances(r.Context(), item)
	if err != nil {
		h.log.Error("failed to fetch balances", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to fetch balances from institution")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":   accounts,
		"item":       item,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}
