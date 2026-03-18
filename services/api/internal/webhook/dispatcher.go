// Package webhook handles outgoing webhook delivery to developer endpoints.
//
// Flow:
//  1. Caller fires Fire(appID, eventType, payload) — non-blocking.
//  2. Dispatcher looks up registered webhooks for that app+event, queues a
//     delivery record in the DB for each, then pushes IDs onto the worker channel.
//  3. Worker goroutines pick up IDs, sign the payload with HMAC-SHA256, and POST
//     to the developer's URL.
//  4. On failure, the delivery is rescheduled with exponential backoff. A separate
//     retry loop polls the DB every 2 minutes for overdue pending deliveries.
//
// Signing format (Stripe-compatible):
//
//	Hound-Signature: t=<unix_ts>,v1=<hmac_sha256_hex>
//	where signature = HMAC-SHA256(secret, "<unix_ts>.<body>")
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/encryption"
	"go.uber.org/zap"
)

const (
	workerCount   = 4
	queueBuffer   = 256
	retryInterval = 2 * time.Minute
	deliverTimeout = 10 * time.Second
)

// Dispatcher sends signed webhook events to developer endpoints.
type Dispatcher struct {
	db    *database.DB
	enc   *encryption.Encryptor
	log   *zap.Logger
	http  *http.Client
	queue chan uuid.UUID
}

// New creates a Dispatcher. Call Start in a goroutine before using Fire.
func New(db *database.DB, enc *encryption.Encryptor, log *zap.Logger) *Dispatcher {
	return &Dispatcher{
		db:   db,
		enc:  enc,
		log:  log,
		http: &http.Client{Timeout: deliverTimeout},
		queue: make(chan uuid.UUID, queueBuffer),
	}
}

// Start launches worker goroutines and the retry loop. Blocks until ctx is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	d.log.Info("webhook dispatcher started", zap.Int("workers", workerCount))

	for i := 0; i < workerCount; i++ {
		go d.worker(ctx)
	}

	// Retry loop: pick up any pending deliveries the workers might have missed
	// (e.g. after a restart, or after a temporary network error).
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.requeuePending(ctx)
		case <-ctx.Done():
			d.log.Info("webhook dispatcher stopped")
			return
		}
	}
}

// Fire queues delivery of eventType to all webhooks registered for appID.
// It is non-blocking and safe to call from request handlers.
func (d *Dispatcher) Fire(ctx context.Context, appID uuid.UUID, eventType string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		d.log.Error("webhook: failed to marshal payload", zap.Error(err))
		return
	}

	webhooks, err := d.db.GetWebhooksForEvent(ctx, appID, eventType)
	if err != nil {
		d.log.Error("webhook: failed to look up registered webhooks", zap.Error(err))
		return
	}
	if len(webhooks) == 0 {
		return
	}

	for _, wh := range webhooks {
		deliveryID, err := d.db.QueueDelivery(ctx, wh.ID, eventType, body)
		if err != nil {
			d.log.Error("webhook: failed to queue delivery", zap.Error(err))
			continue
		}
		// Non-blocking send — if the buffer is full we fall back to the retry loop.
		select {
		case d.queue <- deliveryID:
		default:
			d.log.Warn("webhook: queue full, delivery will be retried by retry loop",
				zap.String("delivery_id", deliveryID.String()))
		}
	}
}

// worker drains the queue channel and delivers each webhook.
func (d *Dispatcher) worker(ctx context.Context) {
	for {
		select {
		case id := <-d.queue:
			d.deliver(ctx, id)
		case <-ctx.Done():
			return
		}
	}
}

// requeuePending picks up any pending deliveries whose next_attempt_at has passed.
func (d *Dispatcher) requeuePending(ctx context.Context) {
	ids, err := d.db.GetPendingDeliveries(ctx, 100)
	if err != nil {
		d.log.Error("webhook: retry loop failed to fetch pending deliveries", zap.Error(err))
		return
	}
	for _, id := range ids {
		select {
		case d.queue <- id:
		default:
		}
	}
}

// deliver fetches a delivery record, signs the payload, and POSTs to the developer URL.
func (d *Dispatcher) deliver(ctx context.Context, deliveryID uuid.UUID) {
	rec, err := d.db.GetDelivery(ctx, deliveryID)
	if err != nil {
		d.log.Error("webhook: failed to fetch delivery record", zap.String("id", deliveryID.String()), zap.Error(err))
		return
	}

	// Decrypt signing secret
	rawSecret, err := d.enc.Decrypt(rec.EncryptedSecret)
	if err != nil {
		d.log.Error("webhook: failed to decrypt signing secret",
			zap.String("webhook_id", rec.WebhookID.String()), zap.Error(err))
		return
	}

	// Build signed request
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := sign(rawSecret, ts, rec.Payload)

	reqCtx, cancel := context.WithTimeout(ctx, deliverTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, rec.WebhookURL, bytes.NewReader(rec.Payload))
	if err != nil {
		d.scheduleRetry(ctx, deliveryID, 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Hound-Signature", fmt.Sprintf("t=%s,v1=%s", ts, sig))
	req.Header.Set("Hound-Webhook-ID", deliveryID.String())
	req.Header.Set("Hound-Event", rec.EventType)
	req.Header.Set("User-Agent", "Hound-Webhooks/1.0")

	resp, err := d.http.Do(req)
	if err != nil {
		d.log.Warn("webhook: delivery failed (network)",
			zap.String("url", rec.WebhookURL),
			zap.String("delivery_id", deliveryID.String()),
			zap.Error(err))
		d.scheduleRetry(ctx, deliveryID, 0, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.db.MarkDelivered(ctx, deliveryID, resp.StatusCode)
		d.log.Info("webhook: delivered",
			zap.String("url", rec.WebhookURL),
			zap.String("event", rec.EventType),
			zap.Int("status", resp.StatusCode))
		return
	}

	// Non-2xx response — schedule retry
	msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
	d.log.Warn("webhook: delivery failed (non-2xx)",
		zap.String("url", rec.WebhookURL),
		zap.String("delivery_id", deliveryID.String()),
		zap.Int("status", resp.StatusCode))
	d.scheduleRetry(ctx, deliveryID, resp.StatusCode, msg)
}

func (d *Dispatcher) scheduleRetry(ctx context.Context, deliveryID uuid.UUID, status int, errMsg string) {
	if err := d.db.ScheduleRetry(ctx, deliveryID, status, errMsg); err != nil {
		d.log.Error("webhook: failed to schedule retry", zap.Error(err))
	}
}

// sign computes HMAC-SHA256(secret, "<timestamp>.<body>") and returns hex.
func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
