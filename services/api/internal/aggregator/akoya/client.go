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
	"mikomo":         true, // Akoya sandbox test institution
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
	// Akoya endpoint: GET /accounts-info/v2/{connector_id}
	data, err := c.get(ctx, item, "/accounts-info/v2/"+item.InstitutionID)
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
	// Akoya transactions are per-account: GET /transactions/v2/{connector_id}/{accountId}
	// Fetch accounts first to get account IDs, then get transactions per account.
	accounts, err := c.GetAccounts(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("akoya get accounts for transactions: %w", err)
	}

	var allTxns []models.Transaction
	for _, acct := range accounts {
		path := fmt.Sprintf("/transactions/v2/%s/%s?startTime=%s&endTime=%s&limit=%d&offset=%d",
			item.InstitutionID, acct.ProviderAccountID,
			start.Format(time.RFC3339), end.Format(time.RFC3339), count, offset)

		data, err := c.get(ctx, item, path)
		if err != nil {
			// Log and skip — one account failing shouldn't block the rest
			continue
		}

		var resp struct {
			Transactions []fdxTransaction `json:"transactions"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		for _, t := range resp.Transactions {
			txns := t.toModel()
			txns.AccountID = acct.ID
			allTxns = append(allTxns, txns)
		}
	}

	return &models.TransactionsResponse{
		Transactions: allTxns,
		TotalCount:   len(allTxns),
	}, nil
}

func (c *Client) GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error) {
	// TODO: implement Akoya identity endpoint
	return nil, fmt.Errorf("akoya identity: not yet implemented")
}

// GetAuthorizationURL builds the Akoya OAuth2 authorization URL.
// Docs: https://docs.akoya.com/reference/get-authorization-code
func (c *Client) GetAuthorizationURL(institutionID, state, redirectURI string) (string, error) {
	// Akoya uses a flat /auth endpoint with connector as a query param (not Keycloak realm routing)
	// e.g. https://sandbox-idp.ddp.akoya.com/auth?connector=mikomo&...
	params := url.Values{}
	params.Set("connector", institutionID) // Akoya param name is "connector", not "connector_id"
	params.Set("response_type", "code")
	params.Set("client_id", c.cfg.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid profile offline_access")
	params.Set("state", state)

	return c.cfg.BaseURL + "/auth?" + params.Encode(), nil
}

// ExchangeCode exchanges an Akoya authorization code for access + ID tokens.
// Docs: https://docs.akoya.com/reference/get-token
func (c *Client) ExchangeCode(ctx context.Context, _ /* institutionID */, code, redirectURI string) (*aggregator.ProviderToken, error) {
	// Akoya token endpoint is flat — no realm/institution in the path
	// Discovery confirms only client_secret_basic (Basic Auth) is supported
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/token",
		bytes.NewBufferString(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)

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
		IDToken      string `json:"id_token"`      // Akoya: use id_token as bearer for data API calls
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("akoya parse token: %w", err)
	}

	// Akoya data API requires the id_token as the bearer token, not the access_token
	bearerToken := result.IDToken
	if bearerToken == "" {
		bearerToken = result.AccessToken // fallback
	}

	token := &aggregator.ProviderToken{
		AccessToken:  bearerToken,
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
	// path already contains the full resource path including connector_id
	// e.g. /accounts-info/v2/mikomo or /transactions/v2/mikomo?...
	dataURL := c.cfg.DataURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataURL, nil)
	if err != nil {
		return nil, err
	}
	// item.ProviderItemID holds the decrypted Akoya access token (decrypted by handler before this call)
	tokenPreview := item.ProviderItemID
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	req.Header.Set("Authorization", "Bearer "+item.ProviderItemID)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("akoya http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("akoya %d url=%s token_prefix=%s body=%s",
			resp.StatusCode, dataURL, tokenPreview, string(body))
	}

	return body, nil
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
		ItemID:            item.ID,
		ProviderAccountID: a.AccountID,
		Name:              a.DisplayName,
		OfficialName:      a.DisplayName,
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
