package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/middleware"
	"github.com/hound-fi/api/internal/models"
	"go.uber.org/zap"
)

// txnCursorPayload is the JSON structure encoded inside a pagination cursor.
type txnCursorPayload struct {
	D string `json:"d"` // date: YYYY-MM-DD
	I string `json:"i"` // transaction UUID
}

func encodeCursor(cp *database.CursorPoint) string {
	if cp == nil {
		return ""
	}
	b, _ := json.Marshal(txnCursorPayload{
		D: cp.Date.Format("2006-01-02"),
		I: cp.ID.String(),
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*database.CursorPoint, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var p txnCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	date, err := time.Parse("2006-01-02", p.D)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return nil, err
	}
	return &database.CursorPoint{Date: date, ID: id}, nil
}

func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	appID, _ := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	accessToken := r.Header.Get("Hound-Access-Token")

	item, err := h.db.GetItemByAccessToken(r.Context(), appID, accessToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ACCESS_TOKEN", "access token is invalid")
		return
	}

	// ── Parse query params ────────────────────────────────────────────────────

	q := r.URL.Query()

	// Default window: last 90 days.
	startDate := time.Now().UTC().AddDate(0, -3, 0)
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
	if c := q.Get("count"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			count = n
		}
	}
	if count > 500 {
		count = 500
	}

	// cursor is nil on the first page; populated on subsequent pages.
	var cursor *database.CursorPoint
	if raw := q.Get("cursor"); raw != "" {
		cursor, err = decodeCursor(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CURSOR", "cursor is malformed or expired")
			return
		}
	}

	if err := h.decryptItem(item); err != nil {
		h.log.Error("failed to decrypt item token", zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token decryption failed")
		return
	}

	// ── Sync from provider on the first page only ─────────────────────────────
	//
	// Subsequent pages (cursor present) skip the provider call — the data was
	// already synced on page 1 and is now served entirely from the DB.

	var dbAccounts []models.Account
	dbAccounts, _ = h.db.GetAccountsByItemID(r.Context(), item.ID)

	if cursor == nil {
		// First page: pull fresh data from the provider and upsert into DB.
		result, provErr := h.agg.GetTransactions(r.Context(), item, startDate, endDate, 0, 0)
		if provErr != nil {
			h.log.Error("failed to fetch transactions from provider",
				zap.String("item_id", item.ID.String()), zap.Error(provErr))
			writeError(w, http.StatusBadGateway, "PROVIDER_ERROR",
				"failed to fetch transactions from institution")
			return
		}

		// Ensure accounts exist in DB (needed to link transactions).
		if len(dbAccounts) == 0 {
			dbAccounts, _ = h.db.UpsertAccounts(r.Context(), item.ID, result.Accounts)
		}

		// Build provider_account_id → DB UUID map from both DB accounts and
		// the accounts the provider returned inline (e.g. Akoya's TransactionsResponse).
		providerToUUID := make(map[string]uuid.UUID, len(dbAccounts))
		for _, a := range dbAccounts {
			providerToUUID[a.ProviderAccountID] = a.ID
		}
		for _, a := range result.Accounts {
			if _, already := providerToUUID[a.ProviderAccountID]; !already {
				// provider account not yet in DB — skip (upsert above should have caught it)
				continue
			}
			providerToUUID[a.ProviderAccountID] = providerToUUID[a.ProviderAccountID]
		}

		persisted, upsertErr := h.db.UpsertTransactions(r.Context(), providerToUUID, result.Transactions)
		if upsertErr != nil {
			h.log.Error("failed to persist transactions",
				zap.String("item_id", item.ID.String()), zap.Error(upsertErr))
		}

		// Fire TRANSACTIONS_SYNC webhook if new/updated transactions were written.
		if h.webhooks != nil && len(persisted) > 0 {
			go h.webhooks.Fire(r.Context(), appID, "TRANSACTIONS_SYNC", map[string]any{
				"webhook_type":     "TRANSACTIONS",
				"webhook_code":     "SYNC_UPDATES_AVAILABLE",
				"item_id":          item.ID,
				"new_transactions": len(persisted),
			})
		}
	}

	// ── Read from DB with keyset pagination ───────────────────────────────────

	txns, total, nextCursor, err := h.db.GetTransactionsByItemIDCursor(
		r.Context(), item.ID, startDate, endDate, count, cursor)
	if err != nil {
		h.log.Error("failed to read transactions from db",
			zap.String("item_id", item.ID.String()), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve transactions")
		return
	}
	if txns == nil {
		txns = []models.Transaction{}
	}

	// ── Build response ────────────────────────────────────────────────────────

	resp := map[string]any{
		"accounts":           dbAccounts,
		"transactions":       txns,
		"total_transactions": total,
		"has_more":           nextCursor != nil,
		"next_cursor":        encodeCursor(nextCursor), // "" when no more pages
		"item":               item,
		"request_id":         r.Header.Get("X-Request-ID"),
	}

	writeJSON(w, http.StatusOK, resp)
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
