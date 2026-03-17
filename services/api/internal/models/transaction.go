package models

import (
	"time"

	"github.com/google/uuid"
)

// Transaction represents a financial transaction.
type Transaction struct {
	ID                uuid.UUID        `json:"transaction_id" db:"id"`
	AccountID         uuid.UUID        `json:"account_id" db:"account_id"`
	Amount            float64          `json:"amount" db:"amount"` // positive = debit, negative = credit
	Currency          string           `json:"iso_currency_code" db:"currency"`
	Date              time.Time        `json:"date" db:"date"`
	AuthorizedDate    *time.Time       `json:"authorized_date,omitempty" db:"authorized_date"`
	Name              string           `json:"name" db:"name"`             // raw description from bank
	MerchantName      string           `json:"merchant_name" db:"merchant_name"` // enriched
	Category          []string         `json:"category" db:"category"`
	CategoryID        string           `json:"category_id" db:"category_id"`
	Pending           bool             `json:"pending" db:"pending"`
	PaymentChannel    string           `json:"payment_channel" db:"payment_channel"` // "online" | "in store" | "other"
	Location          *Location        `json:"location,omitempty" db:"location"`
	PersonalFinance   *PersonalFinance `json:"personal_finance_category,omitempty" db:"personal_finance_category"`
	CreatedAt         time.Time        `json:"created_at" db:"created_at"`
}

// Location is optional enrichment from the merchant.
type Location struct {
	Address     string  `json:"address"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	PostalCode  string  `json:"postal_code"`
	Country     string  `json:"country"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// PersonalFinance is our enriched category (more granular than category[]).
type PersonalFinance struct {
	Primary  string `json:"primary"`
	Detailed string `json:"detailed"`
}

type TransactionsResponse struct {
	Accounts     []Account     `json:"accounts"`
	Transactions []Transaction `json:"transactions"`
	TotalCount   int           `json:"total_transactions"`
	Item         Item          `json:"item"`
}
