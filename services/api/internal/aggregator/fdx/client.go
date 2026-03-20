package fdx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
)

// httpClient wraps http.Client with FDX-specific OAuth and data fetch logic.
type httpClient struct {
	c *http.Client
}

func newHTTPClient() *httpClient {
	return &httpClient{
		c: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// buildAuthURL constructs the OAuth 2.0 authorization URL for the given bank.
// If bank.UsePKCE is true, a PKCE code verifier and challenge are generated and
// the challenge is embedded in the URL. Returns (authURL, codeVerifier, error).
// codeVerifier is "" when PKCE is not used.
func (h *httpClient) buildAuthURL(bank *BankDef, state, redirectURI string) (authURL, codeVerifier string, err error) {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", bank.ClientID())
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(bank.Scopes, " "))
	params.Set("state", state)

	if bank.UsePKCE {
		verifier, err := generateCodeVerifier()
		if err != nil {
			return "", "", fmt.Errorf("fdx: generate code verifier: %w", err)
		}
		params.Set("code_challenge", codeChallenge(verifier))
		params.Set("code_challenge_method", "S256")
		return bank.AuthURL + "?" + params.Encode(), verifier, nil
	}

	return bank.AuthURL + "?" + params.Encode(), "", nil
}

// exchangeCode calls the bank's token endpoint with an authorization_code grant.
// codeVerifier is included in the request when non-empty (PKCE flow).
func (h *httpClient) exchangeCode(ctx context.Context, bank *BankDef, code, redirectURI, codeVerifier string) (*aggregator.ProviderToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", bank.ClientID())

	// Include client_secret if one is configured (some banks allow it alongside PKCE).
	if s := bank.ClientSecret(); s != "" {
		form.Set("client_secret", s)
	}

	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	token, err := h.tokenRequest(ctx, bank.TokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("fdx exchange code (%s): %w", bank.InstitutionID, err)
	}
	return token, nil
}

// refreshToken calls the bank's token endpoint with a refresh_token grant.
func (h *httpClient) refreshToken(ctx context.Context, bank *BankDef, refreshToken string) (*aggregator.ProviderToken, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", bank.ClientID())

	if s := bank.ClientSecret(); s != "" {
		form.Set("client_secret", s)
	}

	token, err := h.tokenRequest(ctx, bank.TokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("fdx refresh token (%s): %w", bank.InstitutionID, err)
	}
	return token, nil
}

// getAccounts calls GET {APIURL}/accounts and returns the decoded account list.
func (h *httpClient) getAccounts(ctx context.Context, bank *BankDef, accessToken string) ([]fdxAccount, error) {
	var resp fdxAccountsResponse
	if err := h.get(ctx, bank, accessToken, "/accounts", &resp); err != nil {
		return nil, fmt.Errorf("fdx get accounts (%s): %w", bank.InstitutionID, err)
	}
	return resp.Accounts, nil
}

// getTransactions calls GET {APIURL}/accounts/{accountID}/transactions with
// startTime, endTime, limit, and offset as query parameters.
func (h *httpClient) getTransactions(ctx context.Context, bank *BankDef, accessToken, accountID string, start, end time.Time, limit, offset int) (*fdxTransactionsResponse, error) {
	path := fmt.Sprintf("/accounts/%s/transactions?startTime=%s&endTime=%s&limit=%d&offset=%d",
		accountID,
		url.QueryEscape(start.UTC().Format(time.RFC3339)),
		url.QueryEscape(end.UTC().Format(time.RFC3339)),
		limit,
		offset,
	)

	var resp fdxTransactionsResponse
	if err := h.get(ctx, bank, accessToken, path, &resp); err != nil {
		return nil, fmt.Errorf("fdx get transactions (%s, acct %s): %w", bank.InstitutionID, accountID, err)
	}
	return &resp, nil
}

// getCustomer calls GET {APIURL}/customers/current.
func (h *httpClient) getCustomer(ctx context.Context, bank *BankDef, accessToken string) (*fdxCustomerResponse, error) {
	var resp fdxCustomerResponse
	if err := h.get(ctx, bank, accessToken, "/customers/current", &resp); err != nil {
		return nil, fmt.Errorf("fdx get customer (%s): %w", bank.InstitutionID, err)
	}
	return &resp, nil
}

// get is the shared GET helper. It sets Authorization and Accept headers, builds
// the full URL from the bank's APIURL, and JSON-decodes the response body into out.
func (h *httpClient) get(ctx context.Context, bank *BankDef, accessToken, path string, out any) error {
	fullURL := strings.TrimRight(bank.APIURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("FDX-Version", bank.FDXVersion)

	resp, err := h.c.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// tokenRequest posts a form-encoded body to tokenURL and returns a ProviderToken.
func (h *httpClient) tokenRequest(ctx context.Context, tokenURL string, form url.Values) (*aggregator.ProviderToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, string(body))
	}

	var result fdxTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
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

// generateCodeVerifier creates a high-entropy PKCE code verifier per RFC 7636.
// 32 random bytes encoded as base64url (no padding) = 43-character string.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge computes the PKCE S256 code challenge for the given verifier.
// challenge = BASE64URL(SHA256(ASCII(verifier)))
func codeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
