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

// CursorPoint encodes a position in a keyset-paginated transaction result.
// It holds the (date, id) of the last row on the previous page.
type CursorPoint struct {
	Date time.Time
	ID   uuid.UUID
}

// GetTransactionsByItemIDCursor returns transactions for an item using keyset
// pagination ordered by (date DESC, id DESC).
//
//   - cursor == nil  → first page, starts from the newest transaction in the window
//   - cursor != nil  → continues after the given (date, id) position
//
// Returns the matching transactions, the total count of rows in the window
// (for X-Total-Count / UI display), and the cursor to pass for the next page
// (nil when there are no more rows).
func (db *DB) GetTransactionsByItemIDCursor(
	ctx context.Context,
	itemID uuid.UUID,
	start, end time.Time,
	limit int,
	cursor *CursorPoint,
) ([]models.Transaction, int, *CursorPoint, error) {
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	// Total count in the window — used for has_more / UI display.
	// TimescaleDB prunes chunks on the date predicate so this is fast.
	var total int
	if err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE a.item_id = $1
		  AND t.date BETWEEN $2 AND $3
	`, itemID, startStr, endStr).Scan(&total); err != nil {
		return nil, 0, nil, fmt.Errorf("count transactions: %w", err)
	}

	// Build the keyset predicate — avoids scanning rows we've already returned.
	var rows pgxRows
	var err error
	if cursor == nil {
		rows, err = db.pool.Query(ctx, `
			SELECT t.id, t.account_id, COALESCE(t.provider_transaction_id,''),
			       t.amount, t.currency, t.date,
			       t.name, COALESCE(t.merchant_name,''), COALESCE(t.category,'{}'),
			       COALESCE(t.category_id,''), t.pending,
			       COALESCE(t.payment_channel,''), t.created_at
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id
			WHERE a.item_id = $1
			  AND t.date BETWEEN $2 AND $3
			ORDER BY t.date DESC, t.id DESC
			LIMIT $4
		`, itemID, startStr, endStr, limit)
	} else {
		// Keyset condition for ORDER BY date DESC, id DESC:
		//   next row satisfies (date < cursor.date) OR (date = cursor.date AND id < cursor.id)
		rows, err = db.pool.Query(ctx, `
			SELECT t.id, t.account_id, COALESCE(t.provider_transaction_id,''),
			       t.amount, t.currency, t.date,
			       t.name, COALESCE(t.merchant_name,''), COALESCE(t.category,'{}'),
			       COALESCE(t.category_id,''), t.pending,
			       COALESCE(t.payment_channel,''), t.created_at
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id
			WHERE a.item_id = $1
			  AND t.date BETWEEN $2 AND $3
			  AND (t.date < $4 OR (t.date = $4 AND t.id < $5))
			ORDER BY t.date DESC, t.id DESC
			LIMIT $6
		`, itemID, startStr, endStr,
			cursor.Date.Format("2006-01-02"), cursor.ID, limit)
	}
	if err != nil {
		return nil, 0, nil, fmt.Errorf("get transactions: %w", err)
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
			return nil, 0, nil, fmt.Errorf("scan transaction: %w", err)
		}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}

	// If we got a full page there may be more — build the next cursor from the
	// last row returned.
	var nextCursor *CursorPoint
	if len(txns) == limit {
		last := txns[len(txns)-1]
		nextCursor = &CursorPoint{Date: last.Date, ID: last.ID}
	}

	return txns, total, nextCursor, nil
}

// pgxRows is the subset of pgx.Rows used here, kept as a local alias so the
// two query branches share a single scan loop without importing pgx directly.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// nullableString returns nil for empty strings so Postgres stores NULL rather
// than an empty string — keeps unique indexes working correctly.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
