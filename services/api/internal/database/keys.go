package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/models"
)

// --- Application management ---

func (db *DB) CreateApplication(ctx context.Context, name, email string) (*models.Application, error) {
	var app models.Application
	err := db.pool.QueryRow(ctx,
		`INSERT INTO applications (name, email)
		 VALUES ($1, $2)
		 RETURNING id, name, email, created_at, updated_at`,
		name, email,
	).Scan(&app.ID, &app.Name, &app.Email, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return &app, nil
}

func (db *DB) GetApplication(ctx context.Context, appID uuid.UUID) (*models.Application, error) {
	var app models.Application
	err := db.pool.QueryRow(ctx,
		`SELECT id, name, email, created_at, updated_at
		 FROM applications
		 WHERE id = $1`,
		appID,
	).Scan(&app.ID, &app.Name, &app.Email, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}
	return &app, nil
}

func (db *DB) ListApplications(ctx context.Context) ([]models.Application, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, name, email, created_at, updated_at
		 FROM applications
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	apps := make([]models.Application, 0)
	for rows.Next() {
		var a models.Application
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// --- API key management ---

// CreateAPIKey generates a new raw API key, stores its SHA-256 hash, and
// returns the record with RawKey set. RawKey is the only time the raw value
// is ever available — it must be shown to the user immediately.
func (db *DB) CreateAPIKey(ctx context.Context, appID uuid.UUID, env, label string) (*models.APIKeyRecord, error) {
	rawKey := "hound_" + env + "_" + generateToken(32)

	var rec models.APIKeyRecord
	err := db.pool.QueryRow(ctx,
		`INSERT INTO api_keys (application_id, key_hash, env, label)
		 VALUES ($2, encode(sha256($1::bytea), 'hex'), $3, $4)
		 RETURNING id, application_id, env, COALESCE(label, ''), created_at`,
		rawKey, appID, env, label,
	).Scan(&rec.ID, &rec.ApplicationID, &rec.Env, &rec.Label, &rec.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	rec.RawKey = rawKey
	return &rec, nil
}

func (db *DB) ListAPIKeys(ctx context.Context, appID uuid.UUID) ([]models.APIKeyRecord, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, application_id, env, COALESCE(label, ''), created_at, revoked_at
		 FROM api_keys
		 WHERE application_id = $1
		 ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	keys := make([]models.APIKeyRecord, 0)
	for rows.Next() {
		var k models.APIKeyRecord
		if err := rows.Scan(&k.ID, &k.ApplicationID, &k.Env, &k.Label, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (db *DB) RevokeAPIKey(ctx context.Context, keyID, appID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`UPDATE api_keys
		 SET revoked_at = NOW()
		 WHERE id = $1 AND application_id = $2 AND revoked_at IS NULL`,
		keyID, appID,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("key not found or already revoked")
	}
	return nil
}
