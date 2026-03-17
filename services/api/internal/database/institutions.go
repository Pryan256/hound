package database

import (
	"context"
	"fmt"

	"github.com/hound-fi/api/internal/models"
)

// SearchInstitutions returns institutions matching query using trigram + full-text search.
// In test env, sandbox institutions are included. In live, they're excluded.
func (db *DB) SearchInstitutions(ctx context.Context, query string, env string, limit int) ([]models.Institution, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	// Combine trigram similarity (catches typos) with full-text (catches word order).
	// Exclude sandbox institutions in live environment.
	rows, err := db.pool.Query(ctx, `
		SELECT id, name, logo_url, primary_color, url, products, status, oauth_only
		FROM institutions
		WHERE status != 'down'
		  AND ($2 = 'test' OR id != 'ins_sandbox')
		  AND (
		      name ILIKE '%' || $1 || '%'
		      OR similarity(name, $1) > 0.2
		      OR name_tsv @@ plainto_tsquery('english', $1)
		  )
		ORDER BY
		    -- Exact prefix match ranked highest
		    CASE WHEN name ILIKE $1 || '%' THEN 0 ELSE 1 END,
		    -- Then trigram similarity
		    similarity(name, $1) DESC,
		    name ASC
		LIMIT $3
	`, query, env, limit)
	if err != nil {
		return nil, fmt.Errorf("search institutions: %w", err)
	}
	defer rows.Close()

	var institutions []models.Institution
	for rows.Next() {
		var inst models.Institution
		var logoURL, primaryColor, url *string
		err := rows.Scan(
			&inst.ID,
			&inst.Name,
			&logoURL,
			&primaryColor,
			&url,
			&inst.Products,
			&inst.Status,
			&inst.OAuthOnly,
		)
		if err != nil {
			return nil, fmt.Errorf("scan institution: %w", err)
		}
		if logoURL != nil {
			inst.LogoURL = *logoURL
		}
		if primaryColor != nil {
			inst.PrimaryColor = *primaryColor
		}
		if url != nil {
			inst.URL = *url
		}
		institutions = append(institutions, inst)
	}

	if institutions == nil {
		institutions = []models.Institution{}
	}

	return institutions, rows.Err()
}

// GetInstitution returns a single institution by ID.
func (db *DB) GetInstitution(ctx context.Context, id string) (*models.Institution, error) {
	var inst models.Institution
	var logoURL, primaryColor, url *string

	err := db.pool.QueryRow(ctx, `
		SELECT id, name, logo_url, primary_color, url, products, status, oauth_only
		FROM institutions
		WHERE id = $1
	`, id).Scan(
		&inst.ID,
		&inst.Name,
		&logoURL,
		&primaryColor,
		&url,
		&inst.Products,
		&inst.Status,
		&inst.OAuthOnly,
	)
	if err != nil {
		return nil, fmt.Errorf("get institution %s: %w", id, err)
	}

	if logoURL != nil {
		inst.LogoURL = *logoURL
	}
	if primaryColor != nil {
		inst.PrimaryColor = *primaryColor
	}
	if url != nil {
		inst.URL = *url
	}

	return &inst, nil
}
