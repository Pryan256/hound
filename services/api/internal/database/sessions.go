package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/models"
)

type OAuthSession struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	UserID        string
	Env           string
	InstitutionID string
	Provider      string
	RedirectURI   string
	RelinkItemID  *uuid.UUID // non-nil when this is a re-authentication of an existing item
}

// CreateOAuthSession records a pending OAuth session and returns the state token.
// Called by the initiate handler before redirecting the user to the bank.
func (db *DB) CreateOAuthSession(ctx context.Context, appID uuid.UUID, linkToken, institutionID, provider string) (state string, err error) {
	state = "st_" + generateToken(24)
	stateExpiry := time.Now().UTC().Add(15 * time.Minute)

	_, err = db.pool.Exec(ctx, `
		INSERT INTO link_sessions (application_id, link_token, institution_id, provider, oauth_state, oauth_state_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, appID, linkToken, institutionID, provider, state, stateExpiry)
	if err != nil {
		return "", fmt.Errorf("create oauth session: %w", err)
	}

	return state, nil
}

// ValidateOAuthState looks up the session by state token, confirms it hasn't expired,
// and returns the session details. Consumes the state (one-time use).
func (db *DB) ValidateOAuthState(ctx context.Context, state string) (*OAuthSession, error) {
	var s OAuthSession
	var linkToken string

	err := db.pool.QueryRow(ctx, `
		SELECT ls.id, ls.application_id, lt.user_id, lt.env, ls.institution_id, ls.provider,
		       COALESCE(lt.redirect_uri, ''), lt.relink_item_id
		FROM link_sessions ls
		JOIN link_tokens lt ON lt.token = ls.link_token
		WHERE ls.oauth_state = $1
		  AND ls.oauth_state_expires_at > NOW()
		  AND ls.item_id IS NULL
	`, state).Scan(&s.ID, &s.ApplicationID, &s.UserID, &s.Env, &s.InstitutionID, &s.Provider,
		&s.RedirectURI, &s.RelinkItemID)
	if err != nil {
		return nil, fmt.Errorf("validate oauth state: %w", err)
	}

	_ = linkToken

	// Expire the state immediately (single use)
	db.pool.Exec(ctx,
		`UPDATE link_sessions SET oauth_state_expires_at = NOW() WHERE oauth_state = $1`, state)

	return &s, nil
}

// CreateItemFromOAuth creates an Item and stores the encrypted provider token,
// then issues a public_token for the Link widget to pass to the developer app.
func (db *DB) CreateItemFromOAuth(
	ctx context.Context,
	session *OAuthSession,
	encryptedAccessToken string,
	encryptedRefreshToken string,
	tokenExpiry *time.Time,
) (item *models.Item, publicToken string, err error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create the Item
	item = &models.Item{}
	err = tx.QueryRow(ctx, `
		INSERT INTO items (application_id, provider, provider_item_id, institution_id, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id, application_id, provider, provider_item_id, institution_id, status, consent_expiry, created_at, updated_at
	`, session.ApplicationID, session.Provider, encryptedAccessToken, session.InstitutionID,
	).Scan(&item.ID, &item.ApplicationID, &item.Provider, &item.ProviderItemID,
		&item.InstitutionID, &item.Status, &item.ConsentExpiry, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("create item: %w", err)
	}

	// Store provider tokens separately (supports future refresh)
	_, err = tx.Exec(ctx, `
		INSERT INTO provider_tokens (item_id, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4)
	`, item.ID, encryptedAccessToken, encryptedRefreshToken, tokenExpiry)
	if err != nil {
		return nil, "", fmt.Errorf("store provider token: %w", err)
	}

	// Link session → item
	publicToken = "public-" + generateToken(32)
	publicTokenExpiry := time.Now().UTC().Add(30 * time.Minute)

	_, err = tx.Exec(ctx, `
		UPDATE link_sessions
		SET item_id = $1, public_token = $2, public_token_expires_at = $3
		WHERE id = $4
	`, item.ID, publicToken, publicTokenExpiry, session.ID)
	if err != nil {
		return nil, "", fmt.Errorf("update link session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	return item, publicToken, nil
}

// UpdateItemFromOAuth re-authenticates an existing item with fresh provider tokens.
// Called by OAuthCallback when session.RelinkItemID is set.
// The item's accounts and transaction history are preserved — only the tokens
// and status change.
func (db *DB) UpdateItemFromOAuth(
	ctx context.Context,
	session *OAuthSession,
	itemID uuid.UUID,
	encryptedAccessToken string,
	encryptedRefreshToken string,
	tokenExpiry *time.Time,
) (item *models.Item, publicToken string, err error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Replace provider tokens and restore item to active.
	// provider_item_id mirrors the encrypted access token (used by GetItemByAccessToken).
	item = &models.Item{}
	err = tx.QueryRow(ctx, `
		UPDATE items
		SET provider_item_id = $2,
		    status           = 'active',
		    updated_at       = NOW()
		WHERE id = $1
		  AND application_id = $3
		RETURNING id, application_id, provider, provider_item_id, institution_id,
		          status, consent_expiry, created_at, updated_at
	`, itemID, encryptedAccessToken, session.ApplicationID,
	).Scan(&item.ID, &item.ApplicationID, &item.Provider, &item.ProviderItemID,
		&item.InstitutionID, &item.Status, &item.ConsentExpiry, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("update item for relink: %w", err)
	}

	// Update the stored provider tokens.
	_, err = tx.Exec(ctx, `
		UPDATE provider_tokens
		SET access_token  = $2,
		    refresh_token = CASE WHEN $3 = '' THEN refresh_token ELSE $3 END,
		    expires_at    = $4,
		    updated_at    = NOW()
		WHERE item_id = $1
	`, itemID, encryptedAccessToken, encryptedRefreshToken, tokenExpiry)
	if err != nil {
		return nil, "", fmt.Errorf("update provider tokens for relink: %w", err)
	}

	// Issue a fresh public_token via the link session so the developer's
	// ExchangePublicToken call works identically to an initial connection.
	publicToken = "public-" + generateToken(32)
	publicTokenExpiry := time.Now().UTC().Add(30 * time.Minute)

	_, err = tx.Exec(ctx, `
		UPDATE link_sessions
		SET item_id = $1, public_token = $2, public_token_expires_at = $3
		WHERE id = $4
	`, itemID, publicToken, publicTokenExpiry, session.ID)
	if err != nil {
		return nil, "", fmt.Errorf("update link session for relink: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	return item, publicToken, nil
}

// GetRedirectURIForLinkToken returns the redirect_uri associated with a link token.
func (db *DB) GetRedirectURIForLinkToken(ctx context.Context, linkToken string) (string, error) {
	var uri string
	err := db.pool.QueryRow(ctx,
		`SELECT COALESCE(redirect_uri, '') FROM link_tokens WHERE token = $1`,
		linkToken,
	).Scan(&uri)
	if err != nil {
		return "", fmt.Errorf("get redirect uri: %w", err)
	}
	return uri, nil
}
