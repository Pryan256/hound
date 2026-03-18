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

	// Default: last 7 years. Wide enough to cover sandbox data (e.g. Mikomo: 2019-2020).
	startDate := time.Now().UTC().AddDate(-7, 0, 0)
	endDate := time.Now().UTC()

	if s := q.Get("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if e := q.Get("end_date"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			endDate = t
		}
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

	if err := h.decryptItem(item); err != nil {
		h.log.Error("failed to decrypt item token", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token decryption failed")
		return
	}

	// Fetch fresh data from the provider.
	result, err := h.agg.GetTransactions(r.Context(), item, startDate, endDate, count, offset)
	if err != nil {
		h.log.Error("failed to fetch transactions", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to fetch transactions from institution")
		return
	}

	// Get persisted accounts (with real UUIDs) to link transactions correctly.
	dbAccounts, err := h.db.GetAccountsByItemID(r.Context(), item.ID)
	if err != nil || len(dbAccounts) == 0 {
		// Accounts not persisted yet — fetch and persist them first.
		fetched, fetchErr := h.agg.GetAccounts(r.Context(), item)
		if fetchErr == nil {
			dbAccounts, _ = h.db.UpsertAccounts(r.Context(), item.ID, fetched)
		}
	}

	// Build provider_account_id → DB UUID map.
	accountUUIDs := make(map[string]uuid.UUID, len(dbAccounts))
	for _, a := range dbAccounts {
		accountUUIDs[a.ProviderAccountID] = a.ID
	}

	// Build provider_account_id → DB UUID map using aggregator result accounts.
	// The Akoya client populates result.Accounts so each transaction's ProviderAccountID
	// can be resolved to a real DB UUID here before upsert.
	providerToUUID := make(map[string]uuid.UUID)
	for _, a := range result.Accounts {
		if dbUUID, ok := accountUUIDs[a.ProviderAccountID]; ok {
			providerToUUID[a.ProviderAccountID] = dbUUID
		}
	}

	// Persist transactions.
	persisted, err := h.db.UpsertTransactions(r.Context(), providerToUUID, result.Transactions)
	if err != nil {
		h.log.Error("failed to persist transactions", zap.String("item_id", item.ID.String()), zap.Error(err))
	}

	// Return persisted transactions if available, otherwise fall back to provider data.
	txns := result.Transactions
	total := result.TotalCount
	if len(persisted) > 0 {
		txns = persisted
		total = len(persisted)
	}

	// Return DB accounts (with real UUIDs) if available.
	returnAccounts := result.Accounts
	if len(dbAccounts) > 0 {
		returnAccounts = dbAccounts
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":           returnAccounts,
		"transactions":       txns,
		"total_transactions": total,
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

	if err := h.decryptItem(item); err != nil {
		h.log.Error("failed to decrypt item token", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token decryption failed")
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
