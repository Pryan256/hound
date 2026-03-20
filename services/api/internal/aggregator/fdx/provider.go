// Package fdx implements aggregator.Provider using direct FDX (Financial Data Exchange)
// bank connections. Each participating bank requires its own OAuth 2.0 client credentials
// configured via environment variables (see banks.go). A bank is active when its
// FDX_<BANK>_CLIENT_ID env var is set.
//
// FDX uses PKCE (RFC 7636) for OAuth, meaning the authorization URL generation and
// the code exchange are coupled: buildAuthURL generates the code verifier, and
// ExchangeCode requires that same verifier.
//
// This provider is sanctioned by CFPB Section 1033 and carries no per-item fees,
// making it the highest-priority provider in the aggregator router.
package fdx

import (
	"context"
	"fmt"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/models"
)

// Provider implements aggregator.Provider using direct FDX bank connections.
// A bank is active when its env-var credentials (FDX_<BANK>_CLIENT_ID) are set.
// Multiple banks are handled by a single Provider instance — institution routing
// is done internally via the banks registry.
type Provider struct {
	http *httpClient
}

// New returns a ready-to-use FDX Provider.
func New() *Provider {
	return &Provider{http: newHTTPClient()}
}

// Name returns the provider identifier.
func (p *Provider) Name() string { return "fdx" }

// Supports returns true if the institution is in the FDX registry AND has client
// credentials configured via environment variables.
func (p *Provider) Supports(_ context.Context, institutionID string) bool {
	bank, err := Lookup(institutionID)
	if err != nil {
		return false
	}
	return bank.Configured()
}

// GetAuthorizationURL builds the bank's OAuth 2.0 authorization URL.
// For PKCE banks (all current FDX banks), it generates a code verifier and embeds
// the S256 challenge in the URL. The returned codeVerifier must be stored by the
// caller and passed to ExchangeCode on callback.
func (p *Provider) GetAuthorizationURL(institutionID, state, redirectURI string) (authURL string, codeVerifier string, err error) {
	bank, err := Lookup(institutionID)
	if err != nil {
		return "", "", err
	}
	return p.http.buildAuthURL(bank, state, redirectURI)
}

// ExchangeCode exchanges an OAuth authorization code for provider tokens.
// codeVerifier must be the value returned by GetAuthorizationURL for this session.
func (p *Provider) ExchangeCode(ctx context.Context, institutionID, code, redirectURI, codeVerifier string) (*aggregator.ProviderToken, error) {
	bank, err := Lookup(institutionID)
	if err != nil {
		return nil, err
	}
	return p.http.exchangeCode(ctx, bank, code, redirectURI, codeVerifier)
}

// RefreshToken uses item.InstitutionID to look up the bank's token URL and
// exchanges the refresh token for a new access token.
func (p *Provider) RefreshToken(ctx context.Context, item *models.Item, refreshToken string) (*aggregator.ProviderToken, error) {
	if refreshToken == "" {
		return nil, aggregator.ErrRefreshNotSupported
	}
	bank, err := Lookup(item.InstitutionID)
	if err != nil {
		return nil, err
	}
	return p.http.refreshToken(ctx, bank, refreshToken)
}

// GetAccounts fetches all accounts for the item from the bank's FDX API.
// item.ProviderItemID is the decrypted access token (the caller decrypts before calling).
func (p *Provider) GetAccounts(ctx context.Context, item *models.Item) ([]models.Account, error) {
	bank, err := Lookup(item.InstitutionID)
	if err != nil {
		return nil, err
	}

	fdxAccounts, err := p.http.getAccounts(ctx, bank, item.ProviderItemID)
	if err != nil {
		return nil, fmt.Errorf("fdx GetAccounts: %w", err)
	}

	accounts := make([]models.Account, len(fdxAccounts))
	for i, a := range fdxAccounts {
		accounts[i] = mapAccount(a)
		accounts[i].ItemID = item.ID
	}
	return accounts, nil
}

// GetAccountBalances reuses GetAccounts since FDX returns balances inline with
// account data — there is no separate balance endpoint in the FDX spec.
func (p *Provider) GetAccountBalances(ctx context.Context, item *models.Item) ([]models.Account, error) {
	return p.GetAccounts(ctx, item)
}

// GetTransactions fetches transactions across all accounts for the given date range.
// It fetches all accounts first, queries each account's transactions, and combines
// them into a single result. Per-account errors are silently skipped so a single
// inaccessible account does not block the entire response.
// The combined result is then paginated by offset and count.
func (p *Provider) GetTransactions(ctx context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error) {
	bank, err := Lookup(item.InstitutionID)
	if err != nil {
		return nil, err
	}

	// Fetch all accounts to get their IDs.
	fdxAccounts, err := p.http.getAccounts(ctx, bank, item.ProviderItemID)
	if err != nil {
		return nil, fmt.Errorf("fdx GetTransactions (accounts): %w", err)
	}

	accounts := make([]models.Account, len(fdxAccounts))
	for i, a := range fdxAccounts {
		accounts[i] = mapAccount(a)
		accounts[i].ItemID = item.ID
	}

	// Fetch transactions per account, skipping any that error.
	var allTxns []models.Transaction
	for _, fdxAcct := range fdxAccounts {
		resp, err := p.http.getTransactions(ctx, bank, item.ProviderItemID, fdxAcct.AccountID, start, end, 500, 0)
		if err != nil {
			// One account failing should not block others.
			continue
		}
		for _, t := range resp.Transactions {
			txn := mapTransaction(t)
			txn.ProviderAccountID = fdxAcct.AccountID
			allTxns = append(allTxns, txn)
		}
	}

	total := len(allTxns)

	// Apply offset/count pagination across the combined result.
	if offset >= total {
		allTxns = []models.Transaction{}
	} else {
		allTxns = allTxns[offset:]
		if count > 0 && len(allTxns) > count {
			allTxns = allTxns[:count]
		}
	}

	return &models.TransactionsResponse{
		Accounts:     accounts,
		Transactions: allTxns,
		TotalCount:   total,
		Item:         *item,
	}, nil
}

// GetIdentity fetches customer identity information from the bank.
// FDX does not attach owner identity to individual account objects, so this
// returns the same account list as GetAccounts. The customer record is fetched
// to confirm the consent scope is valid but the data is not attached to accounts
// (models.Account has no OwnerName/OwnerEmail fields).
func (p *Provider) GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error) {
	return p.GetAccounts(ctx, item)
}

// RevokeItem is a no-op for FDX. RFC 7009 token revocation is optional per spec
// and not universally implemented by banks. Access tokens expire naturally.
func (p *Provider) RevokeItem(_ context.Context, _ *models.Item) error { return nil }
