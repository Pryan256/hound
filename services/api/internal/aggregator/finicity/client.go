// Package finicity implements the aggregator.Provider interface for Finicity (Mastercard Open Banking).
// Finicity is the fallback provider for institutions not covered by Akoya.
// Docs: https://developer.mastercard.com/open-banking-us/documentation
package finicity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/models"
)

const providerName = "finicity"

type Client struct {
	cfg        config.FinicityConfig
	http       *http.Client
	token      string
	tokenExpiry time.Time
	mu         sync.Mutex
}

func New(cfg config.FinicityConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Name() string { return providerName }

// Supports returns true for institutions NOT served by Akoya (Finicity is the catch-all).
// The router calls Akoya.Supports first; if false, Finicity picks it up.
func (c *Client) Supports(_ context.Context, _ string) bool {
	return true // fallback provider
}

func (c *Client) GetAccounts(ctx context.Context, item *models.Item) ([]models.Account, error) {
	token, err := c.appToken(ctx)
	if err != nil {
		return nil, err
	}

	data, err := c.get(ctx, token, fmt.Sprintf("/aggregation/v1/customers/%s/accounts", item.ProviderItemID))
	if err != nil {
		return nil, fmt.Errorf("finicity get accounts: %w", err)
	}

	var resp struct {
		Accounts []finicityAccount `json:"accounts"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("finicity parse accounts: %w", err)
	}

	accounts := make([]models.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		accounts[i] = a.toModel(item)
	}
	return accounts, nil
}

func (c *Client) GetAccountBalances(ctx context.Context, item *models.Item) ([]models.Account, error) {
	token, err := c.appToken(ctx)
	if err != nil {
		return nil, err
	}

	// Finicity has a dedicated live balance endpoint
	data, err := c.get(ctx, token, fmt.Sprintf("/aggregation/v1/customers/%s/accounts/simple", item.ProviderItemID))
	if err != nil {
		return nil, fmt.Errorf("finicity get balances: %w", err)
	}

	var resp struct {
		Accounts []finicityAccount `json:"accounts"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("finicity parse balances: %w", err)
	}

	accounts := make([]models.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		accounts[i] = a.toModel(item)
	}
	return accounts, nil
}

func (c *Client) GetTransactions(ctx context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error) {
	token, err := c.appToken(ctx)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/aggregation/v3/customers/%s/transactions?fromDate=%d&toDate=%d&limit=%d&start=%d",
		item.ProviderItemID,
		start.Unix(),
		end.Unix(),
		count,
		offset+1, // Finicity uses 1-based offset
	)

	data, err := c.get(ctx, token, path)
	if err != nil {
		return nil, fmt.Errorf("finicity get transactions: %w", err)
	}

	var resp struct {
		Transactions []finicityTransaction `json:"transactions"`
		Found        int                   `json:"found"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("finicity parse transactions: %w", err)
	}

	txns := make([]models.Transaction, len(resp.Transactions))
	for i, t := range resp.Transactions {
		txns[i] = t.toModel()
	}

	return &models.TransactionsResponse{
		Transactions: txns,
		TotalCount:   resp.Found,
	}, nil
}

func (c *Client) GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error) {
	// TODO: implement Finicity identity endpoint
	return nil, fmt.Errorf("finicity identity: not yet implemented")
}

// GetAuthorizationURL generates a Finicity Connect URL.
//
// Finicity's Link flow is different from Akoya: for credential-based institutions
// we generate a Connect URL (Finicity's hosted UI). For OAuth-supported institutions
// Finicity handles the bank OAuth internally within Connect.
//
// The caller must have already provisioned a Finicity customer for this user
// (see database.EnsureFinicityCustomer) and passed its ID via institutionID param.
// In this adapter we expect institutionID to be "finicity:{customerId}" format.
func (c *Client) GetAuthorizationURL(institutionID, state, redirectURI string) (string, error) {
	// TODO: Call Finicity Generate Connect URL API
	// POST /connect/v2/generate
	// {customerId, partnerId, redirectUri, state, type: "lite"}
	// Returns {link: "https://connect.finicity.com/..."}
	//
	// This is a server-side call (needs app token) so it can't be done purely
	// client-side. The handler should call this via a separate method.
	// For now, return a placeholder that the handler can detect.
	return "", fmt.Errorf("finicity: GetAuthorizationURL requires async customer setup — use GenerateConnectURL instead")
}

// GenerateConnectURL calls the Finicity API to create a Connect session URL.
// This is the correct entry point for the Finicity OAuth flow.
func (c *Client) GenerateConnectURL(ctx context.Context, customerID, state, redirectURI string) (string, error) {
	appToken, err := c.appToken(ctx)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]any{
		"customerId":  customerID,
		"partnerId":   c.cfg.PartnerID,
		"redirectUri": redirectURI,
		"state":       state,
		"type":        "lite", // "lite" = account connect only, no branding
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/connect/v2/generate", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	c.setHeaders(req, appToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("finicity generate connect url: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("finicity parse connect url: %w", err)
	}

	return result.Link, nil
}

// ExchangeCode is a no-op for Finicity — they use webhook events or redirect params
// to signal completion, not a standard OAuth code exchange.
func (c *Client) ExchangeCode(_ context.Context, _, _ string) (*aggregator.ProviderToken, error) {
	return nil, fmt.Errorf("finicity: use webhook or redirect callback — ExchangeCode not applicable")
}

func (c *Client) RevokeItem(ctx context.Context, item *models.Item) error {
	token, err := c.appToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.cfg.BaseURL+fmt.Sprintf("/aggregation/v1/customers/%s", item.ProviderItemID), nil)
	if err != nil {
		return err
	}
	c.setHeaders(req, token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("finicity revoke: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// appToken returns a cached Finicity app token, refreshing if expired.
func (c *Client) appToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-30*time.Second)) {
		return c.token, nil
	}

	body, _ := json.Marshal(map[string]string{
		"partnerId":     c.cfg.PartnerID,
		"partnerSecret": c.cfg.PartnerSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/aggregation/v2/partners/authentication",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Finicity-App-Key", c.cfg.AppKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("finicity auth: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("finicity parse token: %w", err)
	}

	c.token = result.Token
	c.tokenExpiry = time.Now().Add(90 * time.Minute) // Finicity tokens last 2h
	return c.token, nil
}

func (c *Client) get(ctx context.Context, token, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("finicity http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("finicity %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) setHeaders(req *http.Request, token string) {
	req.Header.Set("Finicity-App-Token", token)
	req.Header.Set("Finicity-App-Key", c.cfg.AppKey)
	req.Header.Set("Accept", "application/json")
}

// --- Finicity response types ---

type finicityAccount struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Number        string   `json:"number"`
	Type          string   `json:"type"`
	Balance       float64  `json:"balance"`
	AvailBalance  *float64 `json:"availableBalance"`
	Currency      string   `json:"currency"`
}

func (a finicityAccount) toModel(item *models.Item) models.Account {
	mask := a.Number
	if len(mask) > 4 {
		mask = mask[len(mask)-4:]
	}
	return models.Account{
		ItemID:  item.ID,
		Name:    a.Name,
		Type:    models.AccountType(normalizeType(a.Type)),
		Mask:    mask,
		Balances: models.Balances{
			Current:   a.Balance,
			Available: a.AvailBalance,
			Currency:  a.Currency,
		},
	}
}

type finicityTransaction struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	PostedDate  int64   `json:"postedDate"`
	Description string  `json:"description"`
	Status      string  `json:"status"` // "active" | "pending"
	Category    string  `json:"categorization.normalizedPayeeName"`
}

func (t finicityTransaction) toModel() models.Transaction {
	return models.Transaction{
		Amount:   -t.Amount, // Finicity: positive = debit; normalize to our convention
		Date:     time.Unix(t.PostedDate, 0).UTC(),
		Name:     t.Description,
		Pending:  t.Status == "pending",
	}
}

func normalizeType(finicityType string) string {
	switch finicityType {
	case "checking", "savings", "moneyMarket", "cd":
		return "depository"
	case "creditCard", "lineOfCredit":
		return "credit"
	case "investment", "brokerageAccount":
		return "investment"
	case "loan", "mortgage":
		return "loan"
	default:
		return "other"
	}
}
