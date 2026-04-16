package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts persistence for webhook subscriptions and delivery records.
// Write methods accept pgx.Tx to guarantee atomicity with audit log writes.
// Read methods accept db.DBTX to work both inside and outside transactions.
type Repository interface {
	// Subscription CRUD (FR-WHK-001, FR-WHK-003)
	Create(ctx context.Context, tx pgx.Tx, s *Subscription, secret string) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Subscription, error)
	List(ctx context.Context, q db.DBTX, systemID uuid.UUID, params ListParams) ([]*Subscription, int64, error)
	Update(ctx context.Context, tx pgx.Tx, s *Subscription) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error

	// Dispatcher operations (FR-WHK-002, FR-WHK-004)

	// FindUndelivered returns (audit_log_id, subscription_id) pairs for audit
	// entries that have active subscriptions but no delivery record yet.
	// The lookback window prevents reprocessing stale events after a long outage.
	FindUndelivered(ctx context.Context, q db.DBTX, limit int) ([][2]uuid.UUID, error)

	// CreateDelivery inserts a single pending webhook_delivery row.
	CreateDelivery(ctx context.Context, tx pgx.Tx, subscriptionID, auditLogID uuid.UUID) error

	// FindPending returns delivery records eligible for immediate dispatch:
	// those with status='pending' or status='failed' with next_retry_at <= now().
	FindPending(ctx context.Context, q db.DBTX, limit int) ([]*PendingDelivery, error)

	// UpdateDelivery persists the result of a delivery attempt.
	UpdateDelivery(ctx context.Context, tx pgx.Tx, deliveryID uuid.UUID, status string, attempts int, nextRetryAt *time.Time, lastResponseCode *int) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed webhook repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func scanSubscription(row pgx.Row) (*Subscription, error) {
	s := &Subscription{}
	err := row.Scan(&s.ID, &s.SystemID, &s.CallbackURL, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning webhook subscription: %w", err)
	}
	return s, nil
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, s *Subscription, secret string) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO webhook_subscription (system_id, callback_url, secret, active)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		s.SystemID, s.CallbackURL, secret, s.Active,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// FK violation on system_id
			return fmt.Errorf("system not found: %w", ErrNotFound)
		}
		return fmt.Errorf("inserting webhook subscription: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Subscription, error) {
	row := q.QueryRow(ctx,
		`SELECT id, system_id, callback_url, active, created_at, updated_at
		 FROM webhook_subscription
		 WHERE id = $1`,
		id,
	)
	return scanSubscription(row)
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX, systemID uuid.UUID, params ListParams) ([]*Subscription, int64, error) {
	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM webhook_subscription WHERE system_id = $1`, systemID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting webhook subscriptions: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT id, system_id, callback_url, active, created_at, updated_at
		 FROM webhook_subscription
		 WHERE system_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		systemID, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing webhook subscriptions: %w", err)
	}
	defer rows.Close()

	result := []*Subscription{}
	for rows.Next() {
		s := &Subscription{}
		if err := rows.Scan(&s.ID, &s.SystemID, &s.CallbackURL, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning webhook subscription: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating webhook subscriptions: %w", err)
	}
	return result, total, nil
}

func (r *pgxRepository) Update(ctx context.Context, tx pgx.Tx, s *Subscription) error {
	err := tx.QueryRow(ctx,
		`UPDATE webhook_subscription
		 SET callback_url = $1, active = $2
		 WHERE id = $3
		 RETURNING updated_at`,
		s.CallbackURL, s.Active, s.ID,
	).Scan(&s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("updating webhook subscription: %w", err)
	}
	return nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, `DELETE FROM webhook_subscription WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting webhook subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindUndelivered returns (audit_log_id, subscription_id) pairs for audit_log entries
// in the past 24 hours that have at least one active subscription but no delivery
// record yet. This drives the dispatcher's enqueue phase (FR-WHK-002).
func (r *pgxRepository) FindUndelivered(ctx context.Context, q db.DBTX, limit int) ([][2]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT al.id, ws.id
		 FROM audit_log al
		 JOIN webhook_subscription ws
		      ON ws.system_id = al.system_id AND ws.active = true
		 LEFT JOIN webhook_delivery wd
		      ON wd.audit_log_id = al.id AND wd.subscription_id = ws.id
		 WHERE wd.id IS NULL
		   AND al.timestamp > NOW() - INTERVAL '24 hours'
		 ORDER BY al.timestamp ASC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("finding undelivered audit entries: %w", err)
	}
	defer rows.Close()

	var result [][2]uuid.UUID
	for rows.Next() {
		var pair [2]uuid.UUID
		if err := rows.Scan(&pair[0], &pair[1]); err != nil {
			return nil, fmt.Errorf("scanning undelivered entry: %w", err)
		}
		result = append(result, pair)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating undelivered entries: %w", err)
	}
	return result, nil
}

func (r *pgxRepository) CreateDelivery(ctx context.Context, tx pgx.Tx, subscriptionID, auditLogID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO webhook_delivery (subscription_id, audit_log_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		subscriptionID, auditLogID,
	)
	if err != nil {
		return fmt.Errorf("creating webhook delivery: %w", err)
	}
	return nil
}

// FindPending returns up to limit delivery records that are ready to dispatch:
// status='pending' or (status='failed' AND next_retry_at <= now()).
// Joins with webhook_subscription for callback_url/secret and audit_log for payload data.
func (r *pgxRepository) FindPending(ctx context.Context, q db.DBTX, limit int) ([]*PendingDelivery, error) {
	rows, err := q.Query(ctx,
		`SELECT wd.id, wd.subscription_id, wd.audit_log_id, wd.attempts,
		        ws.callback_url, ws.secret,
		        al.actor, al.operation, al.resource_type, al.resource_id,
		        al.before_value, al.after_value, al.system_id, al.timestamp
		 FROM webhook_delivery wd
		 JOIN webhook_subscription ws ON ws.id = wd.subscription_id
		 JOIN audit_log al             ON al.id = wd.audit_log_id
		 WHERE wd.status = 'pending'
		    OR (wd.status = 'failed' AND wd.next_retry_at <= NOW())
		 ORDER BY wd.created_at ASC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching pending deliveries: %w", err)
	}
	defer rows.Close()

	var result []*PendingDelivery
	for rows.Next() {
		d := &PendingDelivery{}
		if err := rows.Scan(
			&d.DeliveryID, &d.SubscriptionID, &d.AuditLogID, &d.Attempts,
			&d.CallbackURL, &d.Secret,
			&d.Actor, &d.Operation, &d.ResourceType, &d.ResourceID,
			&d.BeforeValue, &d.AfterValue, &d.SystemID, &d.AuditTimestamp,
		); err != nil {
			return nil, fmt.Errorf("scanning pending delivery: %w", err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pending deliveries: %w", err)
	}
	return result, nil
}

func (r *pgxRepository) UpdateDelivery(ctx context.Context, tx pgx.Tx, deliveryID uuid.UUID, status string, attempts int, nextRetryAt *time.Time, lastResponseCode *int) error {
	_, err := tx.Exec(ctx,
		`UPDATE webhook_delivery
		 SET status = $1, attempts = $2, next_retry_at = $3, last_response_code = $4
		 WHERE id = $5`,
		status, attempts, nextRetryAt, lastResponseCode, deliveryID,
	)
	if err != nil {
		return fmt.Errorf("updating webhook delivery: %w", err)
	}
	return nil
}
