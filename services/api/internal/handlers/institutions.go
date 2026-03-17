package handlers

import (
	"net/http"
	"strconv"

	"github.com/hound-fi/api/internal/middleware"
	"go.uber.org/zap"
)

func (h *Handler) SearchInstitutions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if len(query) < 2 {
		writeError(w, http.StatusBadRequest, "INVALID_FIELD", "query must be at least 2 characters")
		return
	}
	if len(query) > 100 {
		query = query[:100]
	}

	limit := 10
	if l := r.URL.Query().Get("count"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}

	// Env from link token context (sandbox institutions only shown in test)
	env, _ := r.Context().Value(middleware.ContextKeyLinkEnv).(string)
	if env == "" {
		env = "live"
	}

	institutions, err := h.db.SearchInstitutions(r.Context(), query, env, limit)
	if err != nil {
		h.log.Error("institution search failed", zap.String("query", query), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"institutions": institutions,
		"request_id":   r.Header.Get("X-Request-ID"),
	})
}

func (h *Handler) GetInstitution(w http.ResponseWriter, r *http.Request) {
	// institutionID comes from the URL: /link/institutions/{institution_id}
	institutionID := r.URL.Query().Get("institution_id")
	if institutionID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_FIELD", "institution_id is required")
		return
	}

	inst, err := h.db.GetInstitution(r.Context(), institutionID)
	if err != nil {
		h.log.Warn("institution not found", zap.String("institution_id", institutionID), zap.Error(err))
		writeError(w, http.StatusNotFound, "INSTITUTION_NOT_FOUND", "institution not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"institution": inst,
		"request_id":  r.Header.Get("X-Request-ID"),
	})
}
