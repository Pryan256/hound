// Package refresh runs a background goroutine that proactively rotates expiring
// provider tokens before they become invalid. This keeps connected items alive
// without requiring the end-user to re-authenticate.
package refresh

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/encryption"
	"github.com/hound-fi/api/internal/models"
	"github.com/hound-fi/api/internal/webhook"
	"go.uber.org/zap"
)

const (
	// defaultInterval is how often the refresher wakes up and checks for expiring tokens.
	defaultInterval = 15 * time.Minute

	// defaultAhead is how far ahead of expiry we refresh (i.e. refresh tokens
	// expiring within this window, even if they haven't expired yet).
	defaultAhead = 60 * time.Minute
)

// Refresher is a background job that rotates expiring provider tokens.
type Refresher struct {
	db       *database.DB
	agg      *aggregator.Router
	enc      *encryption.Encryptor
	log      *zap.Logger
	webhooks *webhook.Dispatcher
	interval time.Duration
	ahead    time.Duration
}

// New creates a Refresher with default timing parameters.
func New(db *database.DB, agg *aggregator.Router, enc *encryption.Encryptor, log *zap.Logger, webhooks *webhook.Dispatcher) *Refresher {
	return &Refresher{
		db:       db,
		agg:      agg,
		enc:      enc,
		log:      log,
		webhooks: webhooks,
		interval: defaultInterval,
		ahead:    defaultAhead,
	}
}

// Start runs the refresh loop. It blocks until ctx is cancelled — call it in a goroutine.
func (r *Refresher) Start(ctx context.Context) {
	r.log.Info("token refresher started",
		zap.Duration("interval", r.interval),
		zap.Duration("ahead", r.ahead),
	)

	// Run once immediately at startup so freshly-deployed tokens get refreshed
	// even if they were about to expire before the next tick.
	r.run(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.run(ctx)
		case <-ctx.Done():
			r.log.Info("token refresher stopped")
			return
		}
	}
}

// run does one full pass: find expiring tokens, refresh each one.
func (r *Refresher) run(ctx context.Context) {
	expiring, err := r.db.GetExpiringTokens(ctx, r.ahead)
	if err != nil {
		r.log.Error("refresher: failed to fetch expiring tokens", zap.Error(err))
		return
	}
	if len(expiring) == 0 {
		return
	}

	r.log.Info("refresher: found expiring tokens", zap.Int("count", len(expiring)))

	for _, t := range expiring {
		if err := r.refreshOne(ctx, t); err != nil {
			r.log.Error("refresher: failed to refresh token",
				zap.String("item_id", t.ItemID.String()),
				zap.String("provider", t.Provider),
				zap.Error(err),
			)
			// Mark the item as errored so the developer can surface it to the user.
			if markErr := r.db.MarkItemError(ctx, t.ItemID, err.Error()); markErr != nil {
				r.log.Error("refresher: failed to mark item error", zap.Error(markErr))
			}
			// Fire ITEM_ERROR webhook so the developer can prompt re-auth.
			if r.webhooks != nil {
				go r.webhooks.Fire(ctx, t.ApplicationID, "ITEM_ERROR", map[string]any{
					"webhook_type": "ITEM",
					"webhook_code": "ERROR",
					"item_id":      t.ItemID,
					"error":        err.Error(),
				})
			}
		}
	}
}

// refreshOne refreshes the token for a single expiring item.
func (r *Refresher) refreshOne(ctx context.Context, t database.ExpiringToken) error {
	// Decrypt the stored refresh token
	rawRefreshToken, err := r.enc.Decrypt(t.EncRefreshToken)
	if err != nil {
		return fmt.Errorf("decrypt refresh token: %w", err)
	}

	// Build a minimal Item so the router can select the right provider
	item := &models.Item{
		ID:            t.ItemID,
		Provider:      t.Provider,
		InstitutionID: t.InstitutionID,
	}

	// Call the provider's token endpoint
	newToken, err := r.agg.RefreshToken(ctx, item, rawRefreshToken)
	if err != nil {
		if errors.Is(err, aggregator.ErrRefreshNotSupported) {
			// Provider doesn't use refresh tokens — nothing to do, not an error
			return nil
		}
		return fmt.Errorf("provider refresh: %w", err)
	}

	// Encrypt the new access token before storage
	encAccess, err := r.enc.Encrypt(newToken.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt new access token: %w", err)
	}

	// Encrypt the new refresh token if one was issued (some providers rotate it)
	encRefresh := ""
	if newToken.RefreshToken != "" {
		encRefresh, err = r.enc.Encrypt(newToken.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt new refresh token: %w", err)
		}
	}

	// Persist — also syncs items.provider_item_id
	if err := r.db.UpdateProviderToken(ctx, t.ItemID, encAccess, encRefresh, newToken.ExpiresAt); err != nil {
		return fmt.Errorf("store refreshed token: %w", err)
	}

	r.log.Info("refresher: token refreshed",
		zap.String("item_id", t.ItemID.String()),
		zap.String("provider", t.Provider),
		zap.Timep("new_expiry", newToken.ExpiresAt),
	)
	return nil
}
