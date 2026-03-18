package models

import (
	"time"

	"github.com/google/uuid"
)

// Application represents a developer account — the entity that owns API keys,
// items, and financial data for their end users.
type Application struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKeyRecord is the metadata for an API key. RawKey is only populated at
// creation time — we never store it, so it can never be retrieved again.
type APIKeyRecord struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	Env           string     `json:"env"` // "test" | "live"
	Label         string     `json:"label,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`

	// RawKey is set once on creation, never stored, never returned again after that.
	RawKey string `json:"key,omitempty"`
}
