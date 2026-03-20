package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Migrate() error {
	// Migrations run via golang-migrate in the migrate command.
	// This is a no-op in the main server — migrations are a pre-deploy step.
	return nil
}

// --- Link token operations ---

type LinkTokenSession struct {
	ApplicationID uuid.UUID
	UserID        string
	Env           string
}

func (db *DB) ValidateLinkToken(ctx context.Context, token string) (*LinkTokenSession, error) {
	var s LinkTokenSession
	err := db.pool.QueryRow(ctx,
		`SELECT application_id, user_id, env
		 FROM link_tokens
		 WHERE token = $1
		   AND expires_at > NOW()`,
		token,
	).Scan(&s.ApplicationID, &s.UserID, &s.Env)
	if err != nil {
		return nil, fmt.Errorf("validate link token: %w", err)
	}
	return &s, nil
}

// --- API Key operations ---

type APIKey struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	Env           string // "test" | "live"
}

func (db *DB) ValidateAPIKey(ctx context.Context, rawKey string) (*APIKey, error) {
	var key APIKey
	err := db.pool.QueryRow(ctx,
		`SELECT id, application_id, env
		 FROM api_keys
		 WHERE key_hash = encode(sha256($1::bytea), 'hex')
		   AND revoked_at IS NULL`,
		rawKey,
	).Scan(&key.ID, &key.ApplicationID, &key.Env)
	if err != nil {
		return nil, fmt.Errorf("validate api key: %w", err)
	}
	return &key, nil
}

// --- Link token operations ---

func (db *DB) CreateLinkToken(ctx context.Context, appID uuid.UUID, userID string, products []string, env string) (string, time.Time, error) {
	token := "link-" + generateToken(32)
	expiry := time.Now().UTC().Add(30 * time.Minute)

	_, err := db.pool.Exec(ctx,
		`INSERT INTO link_tokens (token, application_id, user_id, products, env, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		token, appID, userID, products, env, expiry,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create link token: %w", err)
	}

	return token, expiry, nil
}

// CreateRelinkToken issues a link token scoped to an existing item.
// When this token is used to complete an OAuth flow, the existing item's
// provider tokens are replaced and its status is restored to active rather
// than a new item being created.
func (db *DB) CreateRelinkToken(ctx context.Context, appID uuid.UUID, userID string, itemID uuid.UUID, env string) (string, time.Time, error) {
	token := "link-" + generateToken(32)
	expiry := time.Now().UTC().Add(30 * time.Minute)

	_, err := db.pool.Exec(ctx,
		`INSERT INTO link_tokens (token, application_id, user_id, products, env, expires_at, relink_item_id)
		 VALUES ($1, $2, $3, '{"transactions","identity"}', $4, $5, $6)`,
		token, appID, userID, env, expiry, itemID,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create relink token: %w", err)
	}

	return token, expiry, nil
}

func (db *DB) ExchangePublicToken(ctx context.Context, appID uuid.UUID, publicToken string) (*models.Item, string, error) {
	// Look up the completed Link session by public token
	var item models.Item
	err := db.pool.QueryRow(ctx,
		`SELECT i.id, i.application_id, i.provider, i.provider_item_id, i.institution_id, i.status, i.consent_expiry, i.created_at, i.updated_at
		 FROM items i
		 JOIN link_sessions ls ON ls.item_id = i.id
		 WHERE ls.public_token = $1
		   AND ls.application_id = $2
		   AND ls.public_token_expires_at > NOW()`,
		publicToken, appID,
	).Scan(&item.ID, &item.ApplicationID, &item.Provider, &item.ProviderItemID,
		&item.InstitutionID, &item.Status, &item.ConsentExpiry, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("exchange public token: %w", err)
	}

	// Issue a durable access token
	accessToken := "access-" + generateToken(48)
	_, err = db.pool.Exec(ctx,
		`INSERT INTO access_tokens (token_hash, item_id, application_id)
		 VALUES (encode(sha256($1::bytea), 'hex'), $2, $3)`,
		accessToken, item.ID, appID,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create access token: %w", err)
	}

	// Expire the public token (single use) — error is non-fatal, token has a short TTL anyway.
	_, _ = db.pool.Exec(ctx,
		`UPDATE link_sessions SET public_token_expires_at = NOW() WHERE public_token = $1`,
		publicToken,
	)

	return &item, accessToken, nil
}

func (db *DB) GetItemByAccessToken(ctx context.Context, appID uuid.UUID, accessToken string) (*models.Item, error) {
	var item models.Item
	err := db.pool.QueryRow(ctx,
		`SELECT i.id, i.application_id, i.provider, i.provider_item_id, i.institution_id, i.status, i.consent_expiry, i.created_at, i.updated_at
		 FROM items i
		 JOIN access_tokens at ON at.item_id = i.id
		 WHERE at.token_hash = encode(sha256($1::bytea), 'hex')
		   AND at.application_id = $2
		   AND at.revoked_at IS NULL`,
		accessToken, appID,
	).Scan(&item.ID, &item.ApplicationID, &item.Provider, &item.ProviderItemID,
		&item.InstitutionID, &item.Status, &item.ConsentExpiry, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get item by access token: %w", err)
	}
	return &item, nil
}

func (db *DB) DeleteItem(ctx context.Context, itemID uuid.UUID) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE items SET status = 'revoked', updated_at = NOW() WHERE id = $1`,
		itemID,
	)
	// Revoke all access tokens for this item — best-effort, non-fatal.
	_, _ = db.pool.Exec(ctx,
		`UPDATE access_tokens SET revoked_at = NOW() WHERE item_id = $1`,
		itemID,
	)
	return err
}

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
