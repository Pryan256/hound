package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
	"go.uber.org/zap"
)

func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	// Parse query params
	q := r.URL.Query()

	startDate, err := time.Parse("2006-01-02", q.Get("start_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_FIELD", "start_date must be YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse("2006-01-02", q.Get("end_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_FIELD", "end_date must be YYYY-MM-DD")
		return
	}

	count := 100
	if q.Get("count") != "" {
		count, _ = strconv.Atoi(q.Get("count"))
		if count > 500 {
			count = 500
		}
	}

	offset := 0
	if q.Get("offset") != "" {
		offset, _ = strconv.Atoi(q.Get("offset"))
	}

	result, err := h.agg.GetTransactions(r.Context(), item, startDate, endDate, count, offset)
	if err != nil {
		h.log.Error("failed to fetch transactions", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to fetch transactions from institution")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":           result.Accounts,
		"transactions":       result.Transactions,
		"total_transactions": result.TotalCount,
		"item":               item,
		"request_id":         r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) GetIdentity(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	identity, err := h.agg.GetIdentity(r.Context(), item)
	if err != nil {
		h.log.Error("failed to fetch identity", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to fetch identity from institution")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":   identity,
		"item":       item,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}
