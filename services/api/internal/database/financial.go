package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/models"
)

// ── Accounts ─────────────────────────────────────────────────────────────────

// UpsertAccounts inserts or updates accounts for an item.
// Conflict key: (item_id, provider_account_id).
// Returns the accounts with their DB-assigned UUIDs populated.
func (db *DB) UpsertAccounts(ctx context.Context, itemID uuid.UUID, accounts []models.Account) ([]models.Account, error) {
	result := make([]models.Account, 0, len(accounts))

	for _, acct := range accounts {
		balancesJSON, err := json.Marshal(acct.Balances)
		if err != nil {
			return nil, fmt.Errorf("marshal balances: %w", err)
		}

		var a models.Account
		var rawBalances []byte

		err = db.pool.QueryRow(ctx, `
			INSERT INTO accounts (item_id, provider_account_id, name, official_name, type, subtype, mask, balances)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (item_id, provider_account_id) WHERE provider_account_id IS NOT NULL
			DO UPDATE SET
				name          = EXCLUDED.name,
				official_name = EXCLUDED.official_name,
				type          = EXCLUDED.type,
				subtype       = EXCLUDED.subtype,
				mask          = EXCLUDED.mask,
				balances      = EXCLUDED.balances,
				updated_at    = NOW()
			RETURNING id, item_id, provider_account_id, name, official_name, type, subtype, mask, balances, created_at, updated_at
		`, itemID, acct.ProviderAccountID, acct.Name, acct.OfficialName,
			string(acct.Type), acct.Subtype, acct.Mask, balancesJSON,
		).Scan(
			&a.ID, &a.ItemID, &a.ProviderAccountID,
			&a.Name, &a.OfficialName, &a.Type, &a.Subtype, &a.Mask,
			&rawBalances, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("upsert account %s: %w", acct.ProviderAccountID, err)
		}

		if err := json.Unmarshal(rawBalances, &a.Balances); err != nil {
			return nil, fmt.Errorf("unmarshal balances: %w", err)
		}

		result = append(result, a)
	}

	return result, nil
}

// GetAccountsByItemID returns all accounts for an item ordered by creation time.
func (db *DB) GetAccountsByItemID(ctx context.Context, itemID uuid.UUID) ([]models.Account, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, item_id, COALESCE(provider_account_id, ''), name, official_name,
		       type, subtype, mask, balances, created_at, updated_at
		FROM accounts
		WHERE item_id = $1
		ORDER BY created_at
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("get accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var rawBalances []byte
		if err := rows.Scan(
			&a.ID, &a.ItemID, &a.ProviderAccountID,
			&a.Name, &a.OfficialName, &a.Type, &a.Subtype, &a.Mask,
			&rawBalances, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		if err := json.Unmarshal(rawBalances, &a.Balances); err != nil {
			return nil, fmt.Errorf("unmarshal balances: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// ── Transactions ──────────────────────────────────────────────────────────────

// UpsertTransactions inserts or updates transactions.
// accountsByProviderID maps provider_account_id → our DB account UUID.
// Conflict key: (account_id, provider_transaction_id, date) — TimescaleDB
// requires the partition key (date) in every unique index.
func (db *DB) UpsertTransactions(
	ctx context.Context,
	accountsByProviderID map[string]uuid.UUID,
	txns []models.Transaction,
) ([]models.Transaction, error) {
	result := make([]models.Transaction, 0, len(txns))

	for _, txn := range txns {
		// Resolve account UUID: prefer ProviderAccountID lookup, fall back to
		// AccountID (already a DB UUID when set by a non-aggregator path).
		accountID, ok := accountsByProviderID[txn.ProviderAccountID]
		if !ok {
			accountID = txn.AccountID
		}
		if accountID == uuid.Nil {
			continue // skip if we can't link to an account
		}

		var t models.Transaction
		err := db.pool.QueryRow(ctx, `
			INSERT INTO transactions
				(account_id, provider_transaction_id, amount, currency, date, name,
				 merchant_name, category, category_id, pending, payment_channel)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (account_id, provider_transaction_id, date)
				WHERE provider_transaction_id IS NOT NULL
			DO UPDATE SET
				amount         = EXCLUDED.amount,
				pending        = EXCLUDED.pending,
				merchant_name  = EXCLUDED.merchant_name
			RETURNING id, account_id, COALESCE(provider_transaction_id,''),
			          amount, currency, date, name,
			          COALESCE(merchant_name,''), COALESCE(category, '{}'),
			          COALESCE(category_id,''), pending,
			          COALESCE(payment_channel,''), created_at
		`, accountID, nullableString(txn.ProviderTransactionID),
			txn.Amount, txn.Currency, txn.Date.Format("2006-01-02"),
			txn.Name, nullableString(txn.MerchantName),
			txn.Category, nullableString(txn.CategoryID),
			txn.Pending, nullableString(txn.PaymentChannel),
		).Scan(
			&t.ID, &t.AccountID, &t.ProviderTransactionID,
			&t.Amount, &t.Currency, &t.Date,
			&t.Name, &t.MerchantName, &t.Category,
			&t.CategoryID, &t.Pending, &t.PaymentChannel,
			&t.CreatedAt,
		)
		if err != nil {
			// Log and skip rather than failing the entire batch
			continue
		}

		result = append(result, t)
	}

	return result, nil
}

// GetTransactionsByItemID returns paginated transactions for all accounts under
// an item within [start, end].
func (db *DB) GetTransactionsByItemID(
	ctx context.Context,
	itemID uuid.UUID,
	start, end time.Time,
	limit, offset int,
) ([]models.Transaction, int, error) {
	// Count
	var total int
	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE a.item_id = $1
		  AND t.date BETWEEN $2 AND $3
	`, itemID, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	rows, err := db.pool.Query(ctx, `
		SELECT t.id, t.account_id, COALESCE(t.provider_transaction_id,''),
		       t.amount, t.currency, t.date,
		       t.name, COALESCE(t.merchant_name,''), COALESCE(t.category,'{}'),
		       COALESCE(t.category_id,''), t.pending,
		       COALESCE(t.payment_channel,''), t.created_at
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE a.item_id = $1
		  AND t.date BETWEEN $2 AND $3
		ORDER BY t.date DESC
		LIMIT $4 OFFSET $5
	`, itemID, start.Format("2006-01-02"), end.Format("2006-01-02"), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get transactions: %w", err)
	}
	defer rows.Close()

	var txns []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.ID, &t.AccountID, &t.ProviderTransactionID,
			&t.Amount, &t.Currency, &t.Date,
			&t.Name, &t.MerchantName, &t.Category,
			&t.CategoryID, &t.Pending,
			&t.PaymentChannel, &t.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, total, nil
}

// nullableString returns nil for empty strings so Postgres stores NULL rather
// than an empty string — keeps unique indexes working correctly.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
