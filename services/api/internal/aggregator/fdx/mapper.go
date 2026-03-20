package fdx

import (
	"time"

	"github.com/hound-fi/api/internal/models"
)

// mapAccount converts an fdxAccount to a Hound models.Account.
// ItemID and DB-assigned ID are left as zero values; the caller fills them in.
func mapAccount(a fdxAccount) models.Account {
	name := a.Nickname
	if name == "" {
		name = a.DisplayName
	}

	current := valueOrZero(a.CurrentBalance)

	return models.Account{
		ProviderAccountID: a.AccountID,
		Name:              name,
		OfficialName:      a.DisplayName,
		Type:              mapAccountType(a.AccountType),
		Subtype:           subtypeFor(a.AccountType),
		Mask:              extractMask(a.AccountNumberDisplay),
		Balances: models.Balances{
			Current:   current,
			Available: a.AvailableBalance,
			Limit:     a.CreditLine,
			Currency:  a.Currency.CurrencyCode,
		},
	}
}

// mapAccountType maps FDX account type strings to Hound's AccountType constants.
func mapAccountType(fdxType string) models.AccountType {
	switch fdxType {
	case "CHECKING", "SAVINGS", "MONEY_MARKET", "CD", "PREPAID":
		return models.AccountTypeDepository
	case "CREDITCARD", "LINE_OF_CREDIT":
		return models.AccountTypeCredit
	case "INVESTMENT", "BROKERAGE":
		return models.AccountTypeInvestment
	case "LOAN", "MORTGAGE":
		return models.AccountTypeLoan
	default:
		return models.AccountTypeDepository
	}
}

// subtypeFor returns a human-readable subtype string for a given FDX account type.
func subtypeFor(fdxType string) string {
	switch fdxType {
	case "CHECKING":
		return "checking"
	case "SAVINGS":
		return "savings"
	case "MONEY_MARKET":
		return "money market"
	case "CD":
		return "cd"
	case "PREPAID":
		return "prepaid"
	case "CREDITCARD":
		return "credit card"
	case "LINE_OF_CREDIT":
		return "line of credit"
	case "INVESTMENT":
		return "investment"
	case "BROKERAGE":
		return "brokerage"
	case "LOAN":
		return "loan"
	case "MORTGAGE":
		return "mortgage"
	default:
		return "other"
	}
}

// mapTransaction converts an fdxTransaction to a Hound models.Transaction.
// ProviderAccountID must be set by the caller after the fact (it's not in the
// transaction response — the account ID is known from the request path).
func mapTransaction(t fdxTransaction) models.Transaction {
	// Prefer postedDate over transactionDate.
	var date time.Time
	if t.PostedDate != nil && !t.PostedDate.IsZero() {
		date = t.PostedDate.Time
	} else if t.TransactionDate != nil && !t.TransactionDate.IsZero() {
		date = t.TransactionDate.Time
	}

	name := t.Description
	if name == "" {
		name = t.Memo
	}

	merchantName := ""
	if t.Merchant != nil {
		merchantName = t.Merchant.Name
	}
	if merchantName == "" {
		merchantName = name
	}

	var category []string
	if t.Merchant != nil && t.Merchant.Category != "" {
		category = []string{t.Merchant.Category}
	}

	channel := mapPaymentChannel(t.PaymentChannel)

	pending := t.Status == "PENDING"

	return models.Transaction{
		ProviderTransactionID: t.TransactionID,
		Amount:                t.Amount, // FDX positive = debit, matches Hound convention
		Currency:              t.Currency.CurrencyCode,
		Date:                  date,
		Name:                  name,
		MerchantName:          merchantName,
		Category:              category,
		Pending:               pending,
		PaymentChannel:        channel,
	}
}

// mapPaymentChannel maps FDX payment channel values to Hound's canonical strings.
func mapPaymentChannel(fdxChannel string) string {
	switch fdxChannel {
	case "IN_STORE":
		return "in store"
	case "ONLINE":
		return "online"
	default:
		return "other"
	}
}

// extractMask returns the last 4 characters of a masked account number display string
// such as "****1234" or "...1234". Returns the full string if it is 4 chars or fewer.
func extractMask(display string) string {
	if len(display) <= 4 {
		return display
	}
	return display[len(display)-4:]
}

// valueOrZero dereferences a *float64, returning 0 if the pointer is nil.
func valueOrZero(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
