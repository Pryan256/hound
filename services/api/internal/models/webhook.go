package models

import (
	"time"

	"github.com/google/uuid"
)

// Webhook is a developer-registered endpoint that receives event notifications.
type Webhook struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	CreatedAt     time.Time `json:"created_at"`

	// Secret is the HMAC signing secret — only populated on creation, never returned again.
	Secret string `json:"secret,omitempty"`
}

// Webhook event types.
const (
	// EventTransactionsSync fires when new or updated transactions are available.
	EventTransactionsSync = "TRANSACTIONS_SYNC"

	// EventItemError fires when an item enters an error state (e.g. token refresh failed).
	EventItemError = "ITEM_ERROR"

	// EventItemLoginRequired fires when the user must re-authenticate with their bank.
	EventItemLoginRequired = "ITEM_LOGIN_REQUIRED"
)
