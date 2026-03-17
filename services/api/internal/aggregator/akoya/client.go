// Package akoya implements the aggregator.Provider interface for Akoya Data Access Network.
// Akoya is a bank-owned OAuth network — no credential scraping, aligns with CFPB 1033.
// Docs: https://docs.akoya.com
package akoya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/models"
)

const providerName = "akoya"

// akoyaInstitutions is the set of FIs available via Akoya.
// In production this should be fetched dynamically from Akoya's institution list API.
var akoyaInstitutions = map[string]bool{
	"chase":          true,
	"bofa":           true,
	"wellsfargo":     true,
	"capitalonebank": true,
	"usbank":         true,
	"citibank":       true,
	"pnc":            true,
	"tdbank":         true,
}

type Client struct {
	cfg    config.AkoyaConfig
	http   *http.Client
}

func New(cfg config.AkoyaConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Name() string { return providerName }

func (c *Client) Supports(_ context.Context, institutionID string) bool {
	return akoyaInstitutions[institutionID]
}

func (c *Client) GetAccounts(ctx context.Context, item *models.Item) ([]models.Account, error) {
	// Akoya FDX endpoint: GET /accounts
	data, err := c.get(ctx, item, "/accounts")
	if err != nil {
		return nil, fmt.Errorf("akoya get accounts: %w", err)
	}

	var resp struct {
		Accounts []fdxAccount `json:"accounts"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("akoya parse accounts: %w", err)
	}

	accounts := make([]models.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		accounts[i] = a.toModel(item)
	}
	return accounts, nil
}

func (c *Client) GetAccountBalances(ctx context.Context, item *models.Item) ([]models.Account, error) {
	// Akoya returns balances inline with accounts
	return c.GetAccounts(ctx, item)
}

func (c *Client) GetTransactions(ctx context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error) {
	path := fmt.Sprintf("/accounts/transactions?startTime=%s&endTime=%s&limit=%d&offset=%d",
		start.Format(time.RFC3339), end.Format(time.RFC3339), count, offset)

	data, err := c.get(ctx, item, path)
	if err != nil {
		return nil, fmt.Errorf("akoya get transactions: %w", err)
	}

	var resp struct {
		Transactions []fdxTransaction `json:"transactions"`
		TotalCount   int              `json:"totalElements"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("akoya parse transactions: %w", err)
	}

	txns := make([]models.Transaction, len(resp.Transactions))
	for i, t := range resp.Transactions {
		txns[i] = t.toModel()
	}

	return &models.TransactionsResponse{
		Transactions: txns,
		TotalCount:   resp.TotalCount,
	}, nil
}

func (c *Client) GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error) {
	// TODO: implement Akoya identity endpoint
	return nil, fmt.Errorf("akoya identity: not yet implemented")
}

// GetAuthorizationURL builds the Akoya OAuth2 authorization URL.
// Akoya uses the connector_id parameter to identify the institution.
// Docs: https://docs.akoya.com/docs/authorization-code-flow
func (c *Client) GetAuthorizationURL(institutionID, state, redirectURI string) (string, error) {
	// Akoya IDP base differs from data API base
	idpBase := c.cfg.BaseURL // e.g. "https://sandbox-idp.ddp.akoya.com"

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", c.cfg.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid profile")
	params.Set("state", state)
	params.Set("connector_id", institutionID) // Akoya-specific: selects the bank

	return idpBase + "/auth?" + params.Encode(), nil
}

// ExchangeCode exchanges an Akoya authorization code for access + ID tokens.
func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*aggregator.ProviderToken, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", redirectURI)
	body.Set("client_id", c.cfg.ClientID)
	body.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/token",
		bytes.NewBufferString(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("akoya token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("akoya token exchange %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("akoya parse token: %w", err)
	}

	token := &aggregator.ProviderToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	if result.ExpiresIn > 0 {
		expiry := time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second)
		token.ExpiresAt = &expiry
	}

	return token, nil
}

func (c *Client) RevokeItem(ctx context.Context, item *models.Item) error {
	// Revoke the OAuth token at Akoya
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.cfg.BaseURL+"/tokens/"+item.ProviderItemID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+item.ProviderItemID)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("akoya revoke: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) get(ctx context.Context, item *models.Item, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	// The provider_item_id holds the Akoya OAuth access token (encrypted at rest in DB)
	req.Header.Set("Authorization", "Bearer "+item.ProviderItemID)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("akoya http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("akoya %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// --- FDX response types (Akoya follows FDX standard) ---

type fdxAccount struct {
	AccountID    string  `json:"accountId"`
	DisplayName  string  `json:"displayName"`
	AccountType  string  `json:"accountType"`
	AccountSubType string `json:"accountSubType"`
	LastFour     string  `json:"lastFourAccountDigits"`
	Balance      float64 `json:"currentBalance"`
	AvailBalance *float64 `json:"availableBalance"`
	Currency     string  `json:"currency"`
}

func (a fdxAccount) toModel(item *models.Item) models.Account {
	return models.Account{
		ItemID:       item.ID,
		Name:         a.DisplayName,
		OfficialName: a.DisplayName,
		Type:         models.AccountType(normalizeAccountType(a.AccountType)),
		Subtype:      a.AccountSubType,
		Mask:         a.LastFour,
		Balances: models.Balances{
			Current:   a.Balance,
			Available: a.AvailBalance,
			Currency:  a.Currency,
		},
	}
}

type fdxTransaction struct {
	TransactionID   string   `json:"transactionId"`
	Amount          float64  `json:"amount"`
	Currency        string   `json:"currency"`
	PostedDate      string   `json:"postedDate"`
	TransactionDate string   `json:"transactionDate"`
	Description     string   `json:"description"`
	Pending         bool     `json:"status"` // FDX uses "PENDING" string
	Category        []string `json:"category"`
}

func (t fdxTransaction) toModel() models.Transaction {
	date, _ := time.Parse("2006-01-02", t.PostedDate)
	return models.Transaction{
		Amount:   t.Amount,
		Currency: t.Currency,
		Date:     date,
		Name:     t.Description,
		Pending:  t.Pending,
		Category: t.Category,
	}
}

func normalizeAccountType(fdxType string) string {
	switch fdxType {
	case "CHECKING", "SAVINGS", "MONEY_MARKET", "CD":
		return "depository"
	case "CREDIT_CARD", "LINE_OF_CREDIT":
		return "credit"
	case "INVESTMENT", "BROKERAGE":
		return "investment"
	case "LOAN", "MORTGAGE":
		return "loan"
	default:
		return "other"
	}
}
