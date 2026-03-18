package handlers

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// portalHTML is the developer portal UI, embedded at compile time.
// It lives in portal.html next to this file to avoid backtick conflicts
// with Go raw string literals (the JS uses template literals).
//
//go:embed portal.html
var portalHTML string

// --- Application endpoints ---

// CreateApplication creates a new developer application.
// POST /management/applications
// Body: { "name": "My App", "email": "dev@example.com" }
func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	if req.Name == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "name and email are required")
		return
	}

	app, err := h.db.CreateApplication(r.Context(), req.Name, req.Email)
	if err != nil {
		h.log.Sugar().Errorf("create application: %v", err)
		if strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "EMAIL_IN_USE", "an application with that email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create application")
		return
	}

	writeJSON(w, http.StatusCreated, app)
}

// ListApplications returns all developer applications.
// GET /management/applications
func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := h.db.ListApplications(r.Context())
	if err != nil {
		h.log.Sugar().Errorf("list applications: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list applications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": apps})
}

// --- API key endpoints ---

// CreateAPIKey generates a new API key for an application.
// POST /management/applications/{appID}/keys
// Body: { "env": "test", "label": "My Key" }
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_APP_ID", "invalid application id")
		return
	}

	var req struct {
		Env   string `json:"env"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if req.Env != "test" && req.Env != "live" {
		writeError(w, http.StatusBadRequest, "INVALID_ENV", `env must be "test" or "live"`)
		return
	}

	key, err := h.db.CreateAPIKey(r.Context(), appID, req.Env, strings.TrimSpace(req.Label))
	if err != nil {
		h.log.Sugar().Errorf("create api key: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create api key")
		return
	}

	// key.RawKey is populated here and will never be retrievable again.
	writeJSON(w, http.StatusCreated, key)
}

// ListAPIKeys returns metadata for all API keys of an application (no raw key values).
// GET /management/applications/{appID}/keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_APP_ID", "invalid application id")
		return
	}

	keys, err := h.db.ListAPIKeys(r.Context(), appID)
	if err != nil {
		h.log.Sugar().Errorf("list api keys: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list api keys")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// RevokeAPIKey permanently revokes an API key.
// DELETE /management/applications/{appID}/keys/{keyID}
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_APP_ID", "invalid application id")
		return
	}
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_KEY_ID", "invalid key id")
		return
	}

	if err := h.db.RevokeAPIKey(r.Context(), keyID, appID); err != nil {
		h.log.Sugar().Errorf("revoke api key: %v", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", "key not found or already revoked")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// Portal serves the developer key management portal HTML page.
// GET /portal
func (h *Handler) Portal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(portalHTML))
}
