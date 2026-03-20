package fdx

import (
	"encoding/json"
	"fmt"
	"time"
)

// fdxAccountsResponse is the top-level response from GET /accounts.
type fdxAccountsResponse struct {
	Accounts []fdxAccount `json:"accounts"`
}

// fdxAccount represents a single FDX account.
type fdxAccount struct {
	AccountID             string      `json:"accountId"`
	AccountType           string      `json:"accountType"`
	DisplayName           string      `json:"displayName"`
	Nickname              string      `json:"nickname"`
	AccountNumberDisplay  string      `json:"accountNumberDisplay"`
	Status                string      `json:"status"`
	Currency              fdxCurrency `json:"currency"`
	CurrentBalance        *float64    `json:"currentBalance"`
	AvailableBalance      *float64    `json:"availableBalance"`
	CreditLine            *float64    `json:"creditLine"`
}

// fdxTransactionsResponse is the top-level response from GET /accounts/{id}/transactions.
type fdxTransactionsResponse struct {
	Transactions []fdxTransaction `json:"transactions"`
	Page         fdxPage          `json:"page"`
}

// fdxTransaction represents a single FDX transaction.
type fdxTransaction struct {
	TransactionID   string       `json:"transactionId"`
	PostedDate      *fdxDate     `json:"postedDate"`
	TransactionDate *fdxDate     `json:"transactionDate"`
	Amount          float64      `json:"amount"`
	TransactionType string       `json:"transactionType"`
	Description     string       `json:"description"`
	Memo            string       `json:"memo"`
	Status          string       `json:"status"`
	PaymentChannel  string       `json:"paymentChannel"`
	Merchant        *fdxMerchant `json:"merchant"`
	Currency        fdxCurrency  `json:"currency"`
}

// fdxMerchant holds merchant details associated with a transaction.
type fdxMerchant struct {
	Name       string `json:"name"`
	MerchantID string `json:"merchantId"`
	Category   string `json:"category"`
}

// fdxPage describes cursor-based pagination in a transactions response.
type fdxPage struct {
	NextOffset    string `json:"nextOffset"`
	TotalElements int    `json:"totalElements"`
	Limit         int    `json:"limit"`
}

// fdxCustomerResponse is the response from GET /customers/current.
type fdxCustomerResponse struct {
	CustomerID string           `json:"customerId"`
	Name       *fdxCustomerName `json:"name"`
	Email      []fdxEmail       `json:"email"`
	Addresses  []fdxAddress     `json:"addresses"`
}

// fdxCustomerName holds the components of a customer's name.
type fdxCustomerName struct {
	First  string `json:"first"`
	Middle string `json:"middle"`
	Last   string `json:"last"`
}

// fdxEmail holds a single email address entry.
type fdxEmail struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// fdxAddress holds a single postal address entry.
type fdxAddress struct {
	Type       string `json:"type"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// fdxTokenResponse is the response from the OAuth 2.0 token endpoint.
type fdxTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// fdxCurrency holds an ISO 4217 currency code.
type fdxCurrency struct {
	CurrencyCode string `json:"currencyCode"`
}

// fdxDate is a time.Time that marshals/unmarshals as an FDX date-only string "2006-01-02".
type fdxDate struct {
	time.Time
}

func (d *fdxDate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("fdxDate: unmarshal string: %w", err)
	}
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("fdxDate: parse %q: %w", s, err)
	}
	d.Time = t
	return nil
}

func (d fdxDate) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format("2006-01-02"))
}
