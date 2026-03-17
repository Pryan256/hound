package aggregator

import (
	"context"
	"time"

	"github.com/hound-fi/api/internal/models"
)

// Provider is the interface every aggregator adapter must implement.
// Adding a new aggregator (e.g. MX) means implementing this interface — nothing else changes.
type Provider interface {
	// Name returns the provider identifier, e.g. "akoya" or "finicity"
	Name() string

	// Supports returns true if this provider can service the given institution.
	// Used by the Router to select the right provider per institution.
	Supports(ctx context.Context, institutionID string) bool

	// GetAccounts returns all accounts for the given item.
	GetAccounts(ctx context.Context, item *models.Item) ([]models.Account, error)

	// GetAccountBalances returns real-time balances (may incur extra cost at some providers).
	GetAccountBalances(ctx context.Context, item *models.Item) ([]models.Account, error)

	// GetTransactions returns transactions for the date range.
	GetTransactions(ctx context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error)

	// GetIdentity returns account holder identity data.
	GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error)

	// RevokeItem revokes consent at the provider level.
	RevokeItem(ctx context.Context, item *models.Item) error

	// --- OAuth flow ---

	// GetAuthorizationURL builds the bank OAuth URL to redirect the user to.
	// state is a random nonce for CSRF protection — must be validated on callback.
	GetAuthorizationURL(institutionID, state, redirectURI string) (string, error)

	// ExchangeCode exchanges an OAuth authorization code for provider tokens.
	// Returns the raw access token (caller is responsible for encrypting before storage).
	ExchangeCode(ctx context.Context, code, redirectURI string) (*ProviderToken, error)
}

// ProviderToken holds the result of a successful OAuth code exchange.
type ProviderToken struct {
	AccessToken  string
	RefreshToken string     // may be empty if provider doesn't issue refresh tokens
	ExpiresAt    *time.Time // nil if provider doesn't specify expiry
}
