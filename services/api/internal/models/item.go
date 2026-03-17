package models

import (
	"time"

	"github.com/google/uuid"
)

// Item represents a connection between an end user and a financial institution.
// Analogous to Plaid's Item.
type Item struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	ApplicationID uuid.UUID  `json:"application_id" db:"application_id"`
	Provider      string     `json:"provider" db:"provider"` // "akoya" | "finicity"
	ProviderItemID string    `json:"-" db:"provider_item_id"` // never expose upstream IDs
	InstitutionID string     `json:"institution_id" db:"institution_id"`
	Status        ItemStatus `json:"status" db:"status"`
	ConsentExpiry *time.Time `json:"consent_expiry,omitempty" db:"consent_expiry"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type ItemStatus string

const (
	ItemStatusActive    ItemStatus = "active"
	ItemStatusPending   ItemStatus = "pending"
	ItemStatusError     ItemStatus = "error"
	ItemStatusRevoked   ItemStatus = "revoked"
)

// ItemError is returned when an item requires attention (re-auth, consent expiry, etc.)
type ItemError struct {
	Code    string `json:"error_code"`
	Message string `json:"error_message"`
	Type    string `json:"error_type"` // "ITEM_ERROR" | "INSTITUTION_ERROR"
}
