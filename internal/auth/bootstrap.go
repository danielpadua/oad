package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BootstrapEntry is one entry from auth.bootstrap_admins in the YAML config.
type BootstrapEntry struct {
	Provider string
	Subject  string
}

// BootstrapRepository is the persistence boundary for the bootstrap routine.
type BootstrapRepository interface {
	// FindEntityID looks up an existing external identity link.
	// Returns ErrNotProvisioned when absent.
	FindEntityID(ctx context.Context, provider, sub string) (uuid.UUID, error)
	// CreateBootstrapUser creates a minimal User entity + external_identity link.
	// Returns the new entity ID.
	CreateBootstrapUser(ctx context.Context, provider, sub string) (uuid.UUID, error)
	// HasAdminRelation reports whether a member_of → oad:admin relation exists.
	HasAdminRelation(ctx context.Context, entityID uuid.UUID) (bool, error)
	// CreateAdminRelation creates the member_of → oad:admin relation + audit log.
	CreateAdminRelation(ctx context.Context, entityID uuid.UUID) error
}

// Bootstrap applies the bootstrap admin list idempotently on startup.
type Bootstrap struct {
	repo BootstrapRepository
}

// NewBootstrap creates a Bootstrap backed by the given repository.
func NewBootstrap(repo BootstrapRepository) *Bootstrap {
	return &Bootstrap{repo: repo}
}

// Apply ensures every entry in entries has an entity + oad:admin relation.
// It is safe to call repeatedly; existing rows are left unchanged.
func (b *Bootstrap) Apply(ctx context.Context, entries []BootstrapEntry) error {
	for _, e := range entries {
		if err := b.applyOne(ctx, e); err != nil {
			return fmt.Errorf("bootstrap %s:%s: %w", e.Provider, e.Subject, err)
		}
	}
	return nil
}

func (b *Bootstrap) applyOne(ctx context.Context, e BootstrapEntry) error {
	entityID, err := b.repo.FindEntityID(ctx, e.Provider, e.Subject)
	if errors.Is(err, ErrNotProvisioned) {
		entityID, err = b.repo.CreateBootstrapUser(ctx, e.Provider, e.Subject)
		if err != nil {
			return fmt.Errorf("create bootstrap user: %w", err)
		}
		slog.Info("bootstrap admin: created entity", "provider", e.Provider, "sub", e.Subject, "entity_id", entityID)
	} else if err != nil {
		return fmt.Errorf("lookup external identity: %w", err)
	}

	hasAdmin, err := b.repo.HasAdminRelation(ctx, entityID)
	if err != nil {
		return fmt.Errorf("check admin relation: %w", err)
	}
	if hasAdmin {
		return nil
	}

	if err := b.repo.CreateAdminRelation(ctx, entityID); err != nil {
		return fmt.Errorf("create admin relation: %w", err)
	}
	slog.Info("bootstrap admin: granted oad:admin", "entity_id", entityID)
	return nil
}

// pgxBootstrapRepository is the production PostgreSQL implementation.
type pgxBootstrapRepository struct {
	pool *pgxpool.Pool
}

// NewBootstrapRepository returns the default PostgreSQL-backed repository.
func NewBootstrapRepository(pool *pgxpool.Pool) BootstrapRepository {
	return &pgxBootstrapRepository{pool: pool}
}

func (r *pgxBootstrapRepository) FindEntityID(ctx context.Context, provider, sub string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT entity_id FROM entity_external_identity WHERE provider_name = $1 AND external_subject = $2`,
		provider, sub,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, ErrNotProvisioned
		}
		return uuid.UUID{}, fmt.Errorf("looking up bootstrap entity for provider %q subject %q: %w", provider, sub, err)
	}
	return id, nil
}

func (r *pgxBootstrapRepository) CreateBootstrapUser(ctx context.Context, provider, sub string) (uuid.UUID, error) {
	props := map[string]any{"active": true, "bootstrap": true, "displayName": "<bootstrap>"}
	propsJSON, _ := json.Marshal(props)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after Commit; error is intentionally ignored

	var entityID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO entity (type_id, external_id, properties)
		 SELECT id, $1, $2::jsonb FROM entity_type_definition WHERE type_name = 'User'
		 RETURNING id`,
		fmt.Sprintf("bootstrap:%s:%s", provider, sub), propsJSON,
	).Scan(&entityID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("insert bootstrap user entity: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO entity_external_identity (entity_id, provider_name, external_subject) VALUES ($1, $2, $3)`,
		entityID, provider, sub,
	); err != nil {
		return uuid.UUID{}, fmt.Errorf("insert external identity: %w", err)
	}

	if err := writeBootstrapAudit(ctx, tx, "create", "entity", entityID); err != nil {
		return uuid.UUID{}, err
	}

	return entityID, tx.Commit(ctx)
}

func (r *pgxBootstrapRepository) HasAdminRelation(ctx context.Context, entityID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM relation rel
			JOIN entity g ON g.id = rel.target_id
			WHERE rel.subject_id = $1
			  AND rel.relation_type = 'member_of'
			  AND g.external_id = 'oad:admin'
		)`,
		entityID,
	).Scan(&exists)
	return exists, err
}

func (r *pgxBootstrapRepository) CreateAdminRelation(ctx context.Context, entityID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after Commit; error is intentionally ignored

	var relationID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO relation (subject_id, relation_type, target_id)
		 SELECT $1, 'member_of', id FROM entity WHERE external_id = 'oad:admin'
		 RETURNING id`,
		entityID,
	).Scan(&relationID)
	if err != nil {
		return fmt.Errorf("insert oad:admin relation: %w", err)
	}

	if err := writeBootstrapAudit(ctx, tx, "create", "relation", relationID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func writeBootstrapAudit(ctx context.Context, tx pgx.Tx, op, resourceType string, resourceID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_log (actor, operation, resource_type, resource_id) VALUES ($1, $2, $3, $4)`,
		"system:bootstrap", op, resourceType, resourceID,
	)
	if err != nil {
		return fmt.Errorf("bootstrap audit log: %w", err)
	}
	return nil
}
