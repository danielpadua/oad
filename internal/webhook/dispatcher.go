package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// pollInterval is how often the dispatcher checks for new work.
	pollInterval = 5 * time.Second

	// maxRetries is the total number of delivery attempts before a delivery
	// is left in 'failed' state permanently.
	maxRetries = 5

	// deliveryTimeout is the HTTP client timeout for each delivery attempt.
	deliveryTimeout = 10 * time.Second

	// enqueueBatchSize and dispatchBatchSize limit how many records are
	// processed per poll cycle to bound memory usage.
	enqueueBatchSize  = 100
	dispatchBatchSize = 50

	// deliveryStatusDelivered and deliveryStatusFailed match the CHECK constraint
	// in webhook_delivery.status.
	deliveryStatusDelivered = "delivered"
	deliveryStatusFailed    = "failed"
)

// Dispatcher is a background worker that enqueues and delivers webhook events.
// It polls the database on a fixed interval for:
//  1. Undelivered audit_log entries with active subscriptions → creates delivery records.
//  2. Pending or retry-eligible delivery records → dispatches HTTP POST to callback URLs.
//
// Each outgoing request is signed with HMAC-SHA256 using the subscription's secret.
// Failed deliveries are retried with exponential backoff up to maxRetries (FR-WHK-004).
type Dispatcher struct {
	pool   *pgxpool.Pool
	repo   Repository
	client *http.Client
	logger *slog.Logger
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(pool *pgxpool.Pool, repo Repository, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		pool:   pool,
		repo:   repo,
		client: &http.Client{Timeout: deliveryTimeout},
		logger: logger,
	}
}

// Run starts the dispatcher loop. It blocks until ctx is cancelled, making it
// suitable for launch as a goroutine alongside the HTTP server.
func (d *Dispatcher) Run(ctx context.Context) {
	d.logger.Info("webhook dispatcher started", "poll_interval", pollInterval)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run one cycle immediately on startup before waiting for the first tick.
	d.cycle(ctx)

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("webhook dispatcher stopped")
			return
		case <-ticker.C:
			d.cycle(ctx)
		}
	}
}

// cycle runs one enqueue + dispatch pass.
func (d *Dispatcher) cycle(ctx context.Context) {
	d.enqueue(ctx)
	d.dispatch(ctx)
}

// enqueue finds audit_log entries that have active subscriptions but no delivery
// record, then creates the delivery rows inside an admin-scoped transaction
// (empty system_id → RLS bypass so all subscriptions are visible).
func (d *Dispatcher) enqueue(ctx context.Context) {
	pairs, err := d.repo.FindUndelivered(ctx, d.pool, enqueueBatchSize)
	if err != nil {
		d.logger.Error("webhook enqueue: finding undelivered entries", "error", err)
		return
	}
	if len(pairs) == 0 {
		return
	}

	// Each (auditLogID, subscriptionID) pair gets its own delivery row.
	// pool.Begin gives us a connection without app.current_system_id set,
	// which matches the RLS "admin mode" (empty = unrestricted).
	tx, beginErr := d.pool.Begin(ctx)
	if beginErr != nil {
		d.logger.Error("webhook enqueue: begin transaction", "error", beginErr)
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is safe to fail if already committed

	created := 0
	for _, pair := range pairs {
		auditLogID, subscriptionID := pair[0], pair[1]
		if createErr := d.repo.CreateDelivery(ctx, tx, subscriptionID, auditLogID); createErr != nil {
			d.logger.Error("webhook enqueue: creating delivery",
				"audit_log_id", auditLogID,
				"subscription_id", subscriptionID,
				"error", createErr,
			)
			// Continue with remaining pairs rather than rolling back the entire batch.
			continue
		}
		created++
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		d.logger.Error("webhook enqueue: committing delivery batch", "error", commitErr)
		return
	}
	if created > 0 {
		d.logger.Info("webhook enqueue: deliveries created", "count", created)
	}
}

// dispatch fetches pending/retry-eligible deliveries and attempts HTTP delivery.
func (d *Dispatcher) dispatch(ctx context.Context) {
	pending, err := d.repo.FindPending(ctx, d.pool, dispatchBatchSize)
	if err != nil {
		d.logger.Error("webhook dispatch: finding pending deliveries", "error", err)
		return
	}
	for _, pd := range pending {
		d.attempt(ctx, pd)
	}
}

// attempt performs a single HTTP delivery for the given pending delivery.
// It updates the delivery record with the outcome, applying exponential backoff
// on failure and marking it permanently failed after maxRetries (FR-WHK-004).
func (d *Dispatcher) attempt(ctx context.Context, pd *PendingDelivery) {
	payload := EventPayload{
		EventID:      pd.DeliveryID,
		EventType:    pd.ResourceType + "." + pd.Operation,
		SystemID:     pd.SystemID,
		ResourceType: pd.ResourceType,
		ResourceID:   pd.ResourceID,
		Actor:        pd.Actor,
		Timestamp:    pd.AuditTimestamp,
		Before:       pd.BeforeValue,
		After:        pd.AfterValue,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("webhook dispatch: marshalling payload",
			"delivery_id", pd.DeliveryID, "error", err)
		return
	}

	sig := computeHMAC(pd.Secret, body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pd.CallbackURL, bytes.NewReader(body))
	if err != nil {
		d.logger.Error("webhook dispatch: building request",
			"delivery_id", pd.DeliveryID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OAD-Delivery", pd.DeliveryID.String())
	req.Header.Set("X-OAD-Signature", "sha256="+sig)

	resp, httpErr := d.client.Do(req)
	newAttempts := pd.Attempts + 1

	var status string
	var lastCode *int
	var nextRetry *time.Time

	if httpErr == nil {
		_ = resp.Body.Close()
		code := resp.StatusCode
		lastCode = &code

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			status = deliveryStatusDelivered
			d.logger.Info("webhook dispatch: delivered",
				"delivery_id", pd.DeliveryID,
				"status_code", code,
				"attempts", newAttempts,
			)
		} else {
			status = deliveryStatusFailed
			d.logger.Warn("webhook dispatch: non-2xx response",
				"delivery_id", pd.DeliveryID,
				"status_code", code,
				"attempts", newAttempts,
			)
		}
	} else {
		status = deliveryStatusFailed
		d.logger.Warn("webhook dispatch: HTTP error",
			"delivery_id", pd.DeliveryID,
			"attempts", newAttempts,
			"error", httpErr,
		)
	}

	// Schedule retry only when below maxRetries and the delivery still failed.
	if status == deliveryStatusFailed && newAttempts < maxRetries {
		t := backoffTime(newAttempts)
		nextRetry = &t
	}

	tx, txErr := d.pool.Begin(ctx)
	if txErr != nil {
		d.logger.Error("webhook dispatch: begin update transaction", "error", txErr)
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is safe to fail if already committed

	if updateErr := d.repo.UpdateDelivery(ctx, tx, pd.DeliveryID, status, newAttempts, nextRetry, lastCode); updateErr != nil {
		d.logger.Error("webhook dispatch: updating delivery",
			"delivery_id", pd.DeliveryID, "error", updateErr)
		return
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		d.logger.Error("webhook dispatch: committing delivery update",
			"delivery_id", pd.DeliveryID, "error", commitErr)
	}
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of body using secret.
// The secret is never included in log output.
func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// backoffTime calculates the next retry timestamp using exponential backoff.
// Base interval is 30 s; each additional attempt doubles the delay, capped at 24 h.
//
//	attempt 1 →  30 s
//	attempt 2 →  60 s
//	attempt 3 → 120 s
//	attempt 4 → 240 s (4 m)
func backoffTime(attempts int) time.Time {
	const base = 30 * time.Second
	const maxBackoff = 24 * time.Hour

	multiplier := time.Duration(1) << (attempts - 1) // 2^(attempts-1)
	backoff := min(base*multiplier, maxBackoff)
	return time.Now().Add(backoff)
}
