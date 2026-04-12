// Package audit provides the audit log service that writes immutable records
// of every write operation within the same database transaction as the
// business operation, ensuring NFR-AUD-001 (no mutation without audit trail).
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/danielpadua/oad/internal/auth"
)

// Operation enumerates the allowed audit operation types, matching the
// CHECK constraint on the audit_log.operation column.
type Operation string

const (
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
)

// Entry represents a single audit log record to be persisted.
type Entry struct {
	Actor        string          // JWT sub or mTLS CN; populated by WriteFromContext.
	Operation    Operation       // create, update, or delete.
	ResourceType string          // e.g., "entity", "relation", "property_overlay".
	ResourceID   uuid.UUID       // Primary key of the affected resource.
	BeforeValue  json.RawMessage // nil on create.
	AfterValue   json.RawMessage // nil on delete.
	SystemID     *string         // nil for global operations.
}

// Service writes audit log entries within an existing database transaction.
// It is stateless and safe for concurrent use.
type Service struct{}

// NewService creates a new audit log service.
func NewService() *Service {
	return &Service{}
}

// Write inserts an audit log entry within the provided transaction.
// The caller is responsible for committing or rolling back the transaction.
// If this INSERT fails, the caller's transaction will fail and roll back,
// guaranteeing that no mutation occurs without a corresponding audit record.
func (s *Service) Write(ctx context.Context, tx pgx.Tx, entry Entry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_log (actor, operation, resource_type, resource_id, before_value, after_value, system_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.Actor,
		string(entry.Operation),
		entry.ResourceType,
		entry.ResourceID,
		entry.BeforeValue,
		entry.AfterValue,
		entry.SystemID,
	)
	if err != nil {
		return fmt.Errorf("writing audit log: %w", err)
	}
	return nil
}

// WriteFromContext is a convenience method that extracts the actor and
// system_id from the authenticated identity in the request context before
// delegating to Write.
func (s *Service) WriteFromContext(ctx context.Context, tx pgx.Tx, entry Entry) error {
	identity := auth.MustIdentityFromContext(ctx)
	entry.Actor = identity.Subject
	if identity.SystemID != "" {
		entry.SystemID = &identity.SystemID
	}
	return s.Write(ctx, tx, entry)
}
