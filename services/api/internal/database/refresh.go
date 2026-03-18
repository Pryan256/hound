package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExpiringToken is a row from provider_tokens joined with its item,
// returned by GetExpiringTokens for the refresher to process.
type ExpiringToken struct {
	ItemID          uuid.UUID
	ApplicationID   uuid.UUID
	Provider        string
	InstitutionID   string
	EncAccessToken  string
	EncRefreshToken string // may be empty — provider doesn't issue refresh tokens
	ExpiresAt       *time.Time
}

// GetExpiringTokens returns active items whose access tokens expire within
// the given threshold (e.g. 60 minutes from now).
// Only returns tokens that have a refresh token — nothing to do without one.
func (db *DB) GetExpiringTokens(ctx context.Context, within time.Duration) ([]ExpiringToken, error) {
	cutoff := time.Now().UTC().Add(within)

	rows, err := db.pool.Query(ctx,
		`SELECT pt.item_id, i.application_id, i.provider, i.institution_id,
		        pt.access_token, COALESCE(pt.refresh_token, ''), pt.expires_at
		 FROM provider_tokens pt
		 JOIN items i ON i.id = pt.item_id
		 WHERE i.status = 'active'
		   AND pt.expires_at IS NOT NULL
		   AND pt.expires_at < $1
		   AND pt.refresh_token IS NOT NULL
		   AND pt.refresh_token != ''
		 ORDER BY pt.expires_at ASC`,
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("get expiring tokens: %w", err)
	}
	defer rows.Close()

	tokens := make([]ExpiringToken, 0)
	for rows.Next() {
		var t ExpiringToken
		if err := rows.Scan(&t.ItemID, &t.ApplicationID, &t.Provider, &t.InstitutionID,
			&t.EncAccessToken, &t.EncRefreshToken, &t.ExpiresAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// UpdateProviderToken atomically replaces the access/refresh tokens for an item
// and keeps items.provider_item_id in sync (it mirrors the access token).
func (db *DB) UpdateProviderToken(ctx context.Context, itemID uuid.UUID, encAccessToken, encRefreshToken string, expiresAt *time.Time) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE provider_tokens
		 SET access_token  = $2,
		     refresh_token = CASE WHEN $3 = '' THEN refresh_token ELSE $3 END,
		     expires_at    = $4,
		     updated_at    = NOW()
		 WHERE item_id = $1`,
		itemID, encAccessToken, encRefreshToken, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("update provider token: %w", err)
	}

	// items.provider_item_id is the encrypted access token used by handlers
	_, err = tx.Exec(ctx,
		`UPDATE items SET provider_item_id = $2, updated_at = NOW() WHERE id = $1`,
		itemID, encAccessToken,
	)
	if err != nil {
		return fmt.Errorf("sync item provider_item_id: %w", err)
	}

	return tx.Commit(ctx)
}

// MarkItemError marks an item as errored with a reason (e.g. refresh failed).
func (db *DB) MarkItemError(ctx context.Context, itemID uuid.UUID, reason string) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE items SET status = 'error', updated_at = NOW() WHERE id = $1`,
		itemID,
	)
	if err != nil {
		return fmt.Errorf("mark item error: %w", err)
	}
	return nil
}
