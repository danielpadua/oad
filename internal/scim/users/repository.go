package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository is the persistence boundary for SCIM Users. The repository
// expresses the (entity, entity_external_identity) two-table layout in
// terms a service can drive without knowing SQL.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, providerName, externalSubject string, props map[string]any) (*StoredUser, error)
	GetByEntityID(ctx context.Context, q db.DBTX, providerName string, id uuid.UUID) (*StoredUser, error)
	Delete(ctx context.Context, tx pgx.Tx, providerName string, id uuid.UUID) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

// makeEntityExternalID composes the entity.external_id value for a SCIM-
// provisioned user. Format: "scim:<provider>:<scim-external-id>". This
// guarantees uniqueness across providers and gives the entity a stable
// reverse-lookup key.
func makeEntityExternalID(providerName, externalSubject string) string {
	return fmt.Sprintf("scim:%s:%s", providerName, externalSubject)
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, providerName, externalSubject string, props map[string]any) (*StoredUser, error) {
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal user properties: %w", err)
	}

	stored := &StoredUser{
		Properties:      props,
		ExternalSubject: externalSubject,
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO entity (type_id, external_id, properties)
		 SELECT id, $1, $2::jsonb FROM entity_type_definition WHERE type_name = 'User'
		 RETURNING id, created_at, updated_at`,
		makeEntityExternalID(providerName, externalSubject), propsJSON,
	).Scan(&stored.EntityID, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("built-in User type not seeded: %w", err)
		}
		return nil, fmt.Errorf("insert user entity: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO entity_external_identity (entity_id, provider_name, external_subject)
		 VALUES ($1, $2, $3)`,
		stored.EntityID, providerName, externalSubject,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("insert external identity: %w", err)
	}

	return stored, nil
}

func (r *pgxRepository) GetByEntityID(ctx context.Context, q db.DBTX, providerName string, id uuid.UUID) (*StoredUser, error) {
	var propsJSON []byte
	stored := &StoredUser{EntityID: id}

	err := q.QueryRow(ctx,
		`SELECT e.properties, ei.external_subject, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition t ON t.id = e.type_id
		 JOIN entity_external_identity ei ON ei.entity_id = e.id
		 WHERE e.id = $1
		   AND t.type_name = 'User'
		   AND ei.provider_name = $2`,
		id, providerName,
	).Scan(&propsJSON, &stored.ExternalSubject, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	if err := json.Unmarshal(propsJSON, &stored.Properties); err != nil {
		return nil, fmt.Errorf("unmarshal user properties: %w", err)
	}
	return stored, nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, providerName string, id uuid.UUID) error {
	// Step 1 — drop the calling provider's link. RowsAffected == 0 means the
	// caller has no claim on this user (either the user doesn't exist, or it
	// belongs to a different provider).
	cmd, err := tx.Exec(ctx,
		`DELETE FROM entity_external_identity
		 WHERE entity_id = $1 AND provider_name = $2`,
		id, providerName,
	)
	if err != nil {
		return fmt.Errorf("delete external identity: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Step 2 — drop the entity if it has no remaining external_identity rows.
	// Cross-IdP merge (Phase D) leaves multiple links; in that case the
	// entity survives until the last link is removed.
	if _, err := tx.Exec(ctx,
		`DELETE FROM entity
		 WHERE id = $1
		   AND NOT EXISTS (SELECT 1 FROM entity_external_identity WHERE entity_id = $1)`,
		id,
	); err != nil {
		return fmt.Errorf("delete user entity: %w", err)
	}
	return nil
}
