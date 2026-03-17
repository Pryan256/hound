package models

import (
	"time"

	"github.com/google/uuid"
)

// Account represents a financial account connected via an Item.
type Account struct {
	ID                uuid.UUID   `json:"account_id" db:"id"`
	ItemID            uuid.UUID   `json:"item_id" db:"item_id"`
	ProviderAccountID string      `json:"provider_account_id" db:"-"` // provider's raw ID, not persisted
	Name              string      `json:"name" db:"name"`
	OfficialName      string      `json:"official_name" db:"official_name"`
	Type              AccountType `json:"type" db:"type"`
	Subtype           string      `json:"subtype" db:"subtype"`
	Mask              string      `json:"mask" db:"mask"` // last 4 digits
	Balances          Balances    `json:"balances" db:"balances"`
	CreatedAt         time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at" db:"updated_at"`
}

type AccountType string

const (
	AccountTypeDepository  AccountType = "depository"
	AccountTypeCredit      AccountType = "credit"
	AccountTypeInvestment  AccountType = "investment"
	AccountTypeLoan        AccountType = "loan"
)

// Balances holds current and available balances.
// Stored as JSONB in Postgres.
type Balances struct {
	Available *float64 `json:"available"` // nil if unavailable
	Current   float64  `json:"current"`
	Limit     *float64 `json:"limit"` // credit accounts
	Currency  string   `json:"iso_currency_code"`
}

type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
	Item     Item      `json:"item"`
}
