package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/models"
)

// --- Webhook management ---

// WebhookRecord includes the encrypted signing secret, used internally by the dispatcher.
type WebhookRecord struct {
	ID               uuid.UUID
	URL              string
	EncryptedSecret  string
}

func (db *DB) CreateWebhook(ctx context.Context, appID uuid.UUID, rawURL string, events []string, encryptedSecret string) (*models.Webhook, error) {
	var w models.Webhook
	err := db.pool.QueryRow(ctx,
		`INSERT INTO webhooks (application_id, url, events, secret_hash, secret_encrypted)
		 VALUES ($1, $2, $3, $4, $4)
		 RETURNING id, application_id, url, events, created_at`,
		appID, rawURL, events, encryptedSecret,
	).Scan(&w.ID, &w.ApplicationID, &w.URL, &w.Events, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}
	return &w, nil
}

func (db *DB) ListWebhooks(ctx context.Context, appID uuid.UUID) ([]models.Webhook, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, application_id, url, events, created_at
		 FROM webhooks
		 WHERE application_id = $1
		 ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	webhooks := make([]models.Webhook, 0)
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.ApplicationID, &w.URL, &w.Events, &w.CreatedAt); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

func (db *DB) DeleteWebhook(ctx context.Context, webhookID, appID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx,
		`DELETE FROM webhooks WHERE id = $1 AND application_id = $2`,
		webhookID, appID,
	)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

// GetWebhooksForEvent returns all webhooks subscribed to a given event type for an application.
func (db *DB) GetWebhooksForEvent(ctx context.Context, appID uuid.UUID, eventType string) ([]WebhookRecord, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, url, COALESCE(secret_encrypted, secret_hash, '')
		 FROM webhooks
		 WHERE application_id = $1
		   AND ($2 = ANY(events) OR 'ALL' = ANY(events))`,
		appID, eventType,
	)
	if err != nil {
		return nil, fmt.Errorf("get webhooks for event: %w", err)
	}
	defer rows.Close()

	records := make([]WebhookRecord, 0)
	for rows.Next() {
		var r WebhookRecord
		if err := rows.Scan(&r.ID, &r.URL, &r.EncryptedSecret); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// --- Delivery queue ---

// DeliveryRecord is a row from webhook_deliveries, used by the dispatcher.
type DeliveryRecord struct {
	ID              uuid.UUID
	WebhookID       uuid.UUID
	WebhookURL      string
	EncryptedSecret string
	EventType       string
	Payload         []byte
	Attempts        int
}

// QueueDelivery inserts a pending delivery record and returns its ID.
func (db *DB) QueueDelivery(ctx context.Context, webhookID uuid.UUID, eventType string, payload []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.pool.QueryRow(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, event_type, payload)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		webhookID, eventType, payload,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("queue delivery: %w", err)
	}
	return id, nil
}

// GetDelivery fetches a delivery record joined with its webhook's URL and secret.
func (db *DB) GetDelivery(ctx context.Context, deliveryID uuid.UUID) (*DeliveryRecord, error) {
	var r DeliveryRecord
	err := db.pool.QueryRow(ctx,
		`SELECT d.id, d.webhook_id, w.url,
		        COALESCE(w.secret_encrypted, w.secret_hash, ''),
		        d.event_type, d.payload, d.attempts
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 WHERE d.id = $1`,
		deliveryID,
	).Scan(&r.ID, &r.WebhookID, &r.WebhookURL, &r.EncryptedSecret,
		&r.EventType, &r.Payload, &r.Attempts)
	if err != nil {
		return nil, fmt.Errorf("get delivery: %w", err)
	}
	return &r, nil
}

// GetPendingDeliveries returns up to limit delivery IDs that are ready to be sent.
func (db *DB) GetPendingDeliveries(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id FROM webhook_deliveries
		 WHERE status = 'pending' AND next_attempt_at <= NOW()
		 ORDER BY next_attempt_at ASC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending deliveries: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MarkDelivered marks a delivery as successfully sent.
func (db *DB) MarkDelivered(ctx context.Context, deliveryID uuid.UUID, httpStatus int) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = 'delivered', delivered_at = NOW(),
		     last_http_status = $2, attempts = attempts + 1
		 WHERE id = $1`,
		deliveryID, httpStatus,
	)
	return err
}

// retryDelays defines how long to wait before each retry attempt.
var retryDelays = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	8 * time.Hour,
}

// ScheduleRetry increments the attempt counter and schedules a future retry.
// After maxAttempts it calls MarkPermFailed instead.
// Returns (true, nil) when the delivery was permanently failed (max retries exhausted).
func (db *DB) ScheduleRetry(ctx context.Context, deliveryID uuid.UUID, httpStatus int, errMsg string) (permFailed bool, err error) {
	// Read current attempt count
	var attempts int
	_ = db.pool.QueryRow(ctx, `SELECT attempts FROM webhook_deliveries WHERE id = $1`, deliveryID).Scan(&attempts)

	if attempts >= len(retryDelays) {
		return true, db.MarkPermFailed(ctx, deliveryID, errMsg)
	}

	nextAttempt := time.Now().UTC().Add(retryDelays[attempts])
	_, err = db.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET attempts = attempts + 1, next_attempt_at = $2,
		     last_http_status = $3, last_error = $4
		 WHERE id = $1`,
		deliveryID, nextAttempt, httpStatus, errMsg,
	)
	return false, err
}

// MarkPermFailed marks a delivery as permanently failed (max retries exhausted).
func (db *DB) MarkPermFailed(ctx context.Context, deliveryID uuid.UUID, errMsg string) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = 'failed', attempts = attempts + 1, last_error = $2
		 WHERE id = $1`,
		deliveryID, errMsg,
	)
	return err
}
