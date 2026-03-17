package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/middleware"
	"go.uber.org/zap"
)

type initiateRequest struct {
	InstitutionID string `json:"institution_id"`
}

type initiateResponse struct {
	OAuthURL  string `json:"oauth_url"`
	RequestID string `json:"request_id"`
}

// OAuthInitiate builds the OAuth URL for the selected institution and returns it
// to the Link widget, which redirects the user's browser there.
//
// Route: POST /link/oauth/initiate?link_token=...
func (h *Handler) OAuthInitiate(w http.ResponseWriter, r *http.Request) {
	appID, ok := r.Context().Value(middleware.ContextKeyApplicationID).(uuid.UUID)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid session")
		return
	}
	linkToken := r.URL.Query().Get("link_token")

	var req initiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InstitutionID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "institution_id is required")
		return
	}

	// Look up institution to confirm it exists and get its provider
	_, err := h.db.GetInstitution(r.Context(), req.InstitutionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INSTITUTION_NOT_FOUND", "institution not found")
		return
	}

	// redirect_uri from the link token (developer-configured), fallback to hosted page
	redirectURI, _ := h.db.GetRedirectURIForLinkToken(r.Context(), linkToken)
	if redirectURI == "" {
		redirectURI = h.cfg.BaseURL() + "/link/oauth/complete"
	}

	// Select provider (Akoya first, Finicity fallback)
	provider, err := h.agg.SelectProvider(r.Context(), req.InstitutionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INSTITUTION_UNSUPPORTED", "no provider available for this institution")
		return
	}

	// Persist the session with a new state token (CSRF protection)
	state, err := h.db.CreateOAuthSession(r.Context(), appID, linkToken, req.InstitutionID, provider.Name())
	if err != nil {
		h.log.Error("failed to create oauth session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to initiate oauth")
		return
	}

	oauthURL, err := provider.GetAuthorizationURL(req.InstitutionID, state, redirectURI)
	if err != nil {
		h.log.Error("failed to build oauth url",
			zap.String("provider", provider.Name()),
			zap.String("institution", req.InstitutionID),
			zap.Error(err))
		writeError(w, http.StatusInternalServerError, "PROVIDER_ERROR", "failed to build authorization URL")
		return
	}

	writeJSON(w, http.StatusOK, initiateResponse{
		OAuthURL:  oauthURL,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

type callbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type callbackResponse struct {
	PublicToken string `json:"public_token"`
	RequestID   string `json:"request_id"`
}

// OAuthCallback handles the POST from the Link widget after the bank redirects back.
// The widget captures the code + state from the URL and sends them here.
//
// Route: POST /link/oauth/callback?link_token=...
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	var req callbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if req.Code == "" || req.State == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "code and state are required")
		return
	}

	// Validate state → get session (single-use, CSRF protection)
	session, err := h.db.ValidateOAuthState(r.Context(), req.State)
	if err != nil {
		h.log.Warn("invalid oauth state", zap.String("state", req.State), zap.Error(err))
		writeError(w, http.StatusBadRequest, "INVALID_STATE", "oauth state is invalid or expired")
		return
	}

	// Provider must match what was selected during initiate
	provider, err := h.agg.SelectProvider(r.Context(), session.InstitutionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PROVIDER_ERROR", "provider unavailable")
		return
	}

	// redirect_uri must be identical to the one used in the authorization request
	redirectURI := session.RedirectURI
	if redirectURI == "" {
		redirectURI = h.cfg.BaseURL() + "/link/oauth/complete"
	}

	// Exchange code for provider tokens
	token, err := provider.ExchangeCode(r.Context(), session.InstitutionID, req.Code, redirectURI)
	if err != nil {
		h.log.Error("oauth code exchange failed",
			zap.String("provider", provider.Name()),
			zap.String("institution", session.InstitutionID),
			zap.Error(err))
		writeError(w, http.StatusBadGateway, "PROVIDER_ERROR", "failed to exchange authorization code")
		return
	}

	// Encrypt tokens before storage — never store plaintext provider credentials
	encAccessToken, err := h.enc.Encrypt(token.AccessToken)
	if err != nil {
		h.log.Error("failed to encrypt access token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token storage failed")
		return
	}

	encRefreshToken := ""
	if token.RefreshToken != "" {
		encRefreshToken, err = h.enc.Encrypt(token.RefreshToken)
		if err != nil {
			h.log.Error("failed to encrypt refresh token", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "token storage failed")
			return
		}
	}

	// Create Item, store tokens, issue public_token
	_, publicToken, err := h.db.CreateItemFromOAuth(
		r.Context(), session, encAccessToken, encRefreshToken, token.ExpiresAt)
	if err != nil {
		h.log.Error("failed to create item", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to complete connection")
		return
	}

	h.log.Info("item created via oauth",
		zap.String("institution", session.InstitutionID),
		zap.String("provider", provider.Name()))

	writeJSON(w, http.StatusOK, callbackResponse{
		PublicToken: publicToken,
		RequestID:   r.Header.Get("X-Request-ID"),
	})
}
