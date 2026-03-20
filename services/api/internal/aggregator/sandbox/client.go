// Package sandbox implements a deterministic fake aggregator for test-mode API keys.
//
// No external calls are made. All data is generated from a seeded PRNG keyed on
// the item's UUID, so the same item always returns the same accounts and transactions.
//
// Link flow:
//
//	GetAuthorizationURL returns the redirectURI with ?code=sandbox_ok&state=<state>
//	already appended, so the browser skips straight to OAuthComplete — no bank page.
package sandbox

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"net/url"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/models"
)

// InstitutionID is the stable identifier for the sandbox institution.
const InstitutionID = "ins_sandbox"

// sandboxCode is the fake OAuth authorization code exchanged during the sandbox flow.
const sandboxCode = "sandbox_ok"

// Client is the sandbox aggregator — implements aggregator.Provider.
type Client struct{}

// New returns a ready-to-use sandbox Client.
func New() *Client { return &Client{} }

func (c *Client) Name() string { return "sandbox" }

func (c *Client) Supports(_ context.Context, institutionID string) bool {
	return institutionID == InstitutionID
}

// ── Data endpoints ────────────────────────────────────────────────────────────

func (c *Client) GetAccounts(_ context.Context, item *models.Item) ([]models.Account, error) {
	return accounts(item.ID.String()), nil
}

func (c *Client) GetAccountBalances(_ context.Context, item *models.Item) ([]models.Account, error) {
	return accounts(item.ID.String()), nil
}

func (c *Client) GetTransactions(_ context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error) {
	accts := accounts(item.ID.String())
	txns := transactions(item.ID.String(), accts, start, end)

	total := len(txns)
	if offset >= total {
		txns = []models.Transaction{}
	} else {
		txns = txns[offset:]
		if count > 0 && len(txns) > count {
			txns = txns[:count]
		}
	}

	return &models.TransactionsResponse{
		Accounts:     accts,
		Transactions: txns,
		TotalCount:   total,
		Item:         *item,
	}, nil
}

func (c *Client) GetIdentity(_ context.Context, item *models.Item) ([]models.Account, error) {
	return accounts(item.ID.String()), nil
}

func (c *Client) RevokeItem(_ context.Context, _ *models.Item) error { return nil }

// ── OAuth flow ────────────────────────────────────────────────────────────────

// GetAuthorizationURL returns the redirectURI pre-loaded with code+state so
// the Link widget lands directly on the OAuth complete page without visiting a bank.
func (c *Client) GetAuthorizationURL(_, state, redirectURI string) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", fmt.Errorf("sandbox: parse redirect uri: %w", err)
	}
	q := u.Query()
	q.Set("code", sandboxCode)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode validates the sandbox code and returns a synthetic provider token.
// The token never expires so the background refresher leaves sandbox items alone.
func (c *Client) ExchangeCode(_ context.Context, _, code, _ string) (*aggregator.ProviderToken, error) {
	if code != sandboxCode {
		return nil, fmt.Errorf("sandbox: invalid code %q", code)
	}
	return &aggregator.ProviderToken{
		AccessToken: "sandbox_access_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		// No RefreshToken, no ExpiresAt — token lives forever.
	}, nil
}

// RefreshToken is a no-op for sandbox — tokens never expire.
func (c *Client) RefreshToken(_ context.Context, _ string) (*aggregator.ProviderToken, error) {
	return nil, aggregator.ErrRefreshNotSupported
}

// ── Fixture data ──────────────────────────────────────────────────────────────

// accounts returns 3 fixed accounts whose ProviderAccountIDs are deterministic
// per item (so repeated calls always get the same accounts into the DB).
func accounts(itemPrefix string) []models.Account {
	pfx := itemPrefix
	if len(pfx) > 8 {
		pfx = pfx[:8]
	}

	avail1 := 1842.55
	avail2 := 8450.00
	limit := 5000.00

	return []models.Account{
		{
			ProviderAccountID: pfx + "-chk",
			Name:              "Sandbox Checking",
			OfficialName:      "Hound Sandbox Checking Account",
			Type:              models.AccountTypeDepository,
			Subtype:           "checking",
			Mask:              "2314",
			Balances: models.Balances{
				Available: &avail1,
				Current:   1842.55,
				Currency:  "USD",
			},
		},
		{
			ProviderAccountID: pfx + "-sav",
			Name:              "Sandbox Savings",
			OfficialName:      "Hound Sandbox Savings Account",
			Type:              models.AccountTypeDepository,
			Subtype:           "savings",
			Mask:              "5873",
			Balances: models.Balances{
				Available: &avail2,
				Current:   8450.00,
				Currency:  "USD",
			},
		},
		{
			ProviderAccountID: pfx + "-cc",
			Name:              "Sandbox Credit Card",
			OfficialName:      "Hound Sandbox Visa Credit Card",
			Type:              models.AccountTypeCredit,
			Subtype:           "credit card",
			Mask:              "9012",
			Balances: models.Balances{
				Current:  432.10, // positive = amount owed
				Limit:    &limit,
				Currency: "USD",
			},
		},
	}
}

// merchant is a fixture entry for transaction generation.
type merchant struct {
	name     string
	category []string
	amount   float64 // base amount; slightly randomised per transaction
	channel  string
}

var merchants = []merchant{
	{"Whole Foods Market", []string{"Food and Drink", "Groceries"}, 73.42, "in store"},
	{"Trader Joe's", []string{"Food and Drink", "Groceries"}, 51.18, "in store"},
	{"Target", []string{"Shops", "Department Stores"}, 89.27, "in store"},
	{"Amazon", []string{"Shops", "Online Marketplaces"}, 42.99, "online"},
	{"Netflix", []string{"Recreation", "Streaming Services"}, 15.49, "online"},
	{"Spotify", []string{"Recreation", "Streaming Services"}, 9.99, "online"},
	{"Starbucks", []string{"Food and Drink", "Coffee Shop"}, 6.75, "in store"},
	{"Chipotle Mexican Grill", []string{"Food and Drink", "Restaurants"}, 13.45, "in store"},
	{"Uber", []string{"Travel", "Ride Share"}, 18.60, "online"},
	{"Shell", []string{"Travel", "Gas Stations"}, 54.30, "in store"},
	{"CVS Pharmacy", []string{"Shops", "Pharmacies"}, 22.14, "in store"},
	{"Home Depot", []string{"Shops", "Home Improvement"}, 67.89, "in store"},
	{"Costco", []string{"Shops", "Warehouse Stores"}, 134.56, "in store"},
	{"Apple", []string{"Shops", "Electronics"}, 9.99, "online"},
	{"Google", []string{"Service", "Digital Services"}, 2.99, "online"},
	{"AT&T", []string{"Service", "Telecommunications"}, 85.00, "online"},
	{"Verizon", []string{"Service", "Telecommunications"}, 79.99, "online"},
	{"PG&E", []string{"Service", "Utilities"}, 112.43, "online"},
	{"Adobe Creative Cloud", []string{"Service", "Software"}, 54.99, "online"},
	{"Dropbox", []string{"Service", "Software"}, 9.99, "online"},
	{"McDonald's", []string{"Food and Drink", "Restaurants"}, 8.49, "in store"},
	{"Walgreens", []string{"Shops", "Pharmacies"}, 17.32, "in store"},
	{"Lyft", []string{"Travel", "Ride Share"}, 14.80, "online"},
	{"Walmart", []string{"Shops", "Supermarkets and Groceries"}, 58.33, "in store"},
	{"Safeway", []string{"Food and Drink", "Groceries"}, 62.77, "in store"},
}

// transactions generates a deterministic set of transactions for a date range.
// The same item ID always produces the same transactions for a given window.
func transactions(itemPrefix string, accts []models.Account, start, end time.Time) []models.Transaction {
	// Seed the PRNG with a hash of the item prefix for determinism.
	h := fnv.New64a()
	_, _ = h.Write([]byte(itemPrefix))
	rng := rand.New(rand.NewSource(int64(h.Sum64())))

	checkingProvID := accts[0].ProviderAccountID
	creditProvID := accts[2].ProviderAccountID

	// Normalise to midnight UTC to avoid time-of-day drift.
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	var txns []models.Transaction
	idx := 0

	for !day.After(endDay) {
		// ~65 % chance of at least one transaction per day
		if rng.Float64() < 0.35 {
			day = day.AddDate(0, 0, 1)
			idx++
			continue
		}

		m := merchants[rng.Intn(len(merchants))]

		// Randomise amount ±20 %
		variation := 0.80 + rng.Float64()*0.40
		amount := math.Round(m.amount*variation*100) / 100

		// 30 % of transactions go on the credit card
		provAcctID := checkingProvID
		if rng.Float64() < 0.30 {
			provAcctID = creditProvID
		}

		txns = append(txns, models.Transaction{
			ProviderTransactionID: fmt.Sprintf("sandbox_%s_%d", itemPrefix[:min8(itemPrefix)], idx),
			ProviderAccountID:     provAcctID,
			Amount:                amount,
			Currency:              "USD",
			Date:                  day,
			Name:                  m.name,
			MerchantName:          m.name,
			Category:              m.category,
			Pending:               false,
			PaymentChannel:        m.channel,
		})

		day = day.AddDate(0, 0, 1)
		idx++
	}

	return txns
}

func min8(s string) int {
	if len(s) < 8 {
		return len(s)
	}
	return 8
}
