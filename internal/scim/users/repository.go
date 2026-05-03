package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	List(ctx context.Context, q db.DBTX, providerName, filterSQL string, filterArgs []any, offset, limit int) (results []*StoredUser, total int, err error)
	Replace(ctx context.Context, tx pgx.Tx, providerName string, id uuid.UUID, props map[string]any) (updatedAt time.Time, err error)
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

// List returns a page of users belonging to the calling provider together
// with the total count for pagination. filterSQL is an empty string when
// no filter is requested; otherwise it is a WHERE-fragment produced by
// FilterToSQL using the table aliases `e` (entity) and `ei`
// (entity_external_identity).
func (r *pgxRepository) List(
	ctx context.Context, q db.DBTX,
	providerName, filterSQL string, filterArgs []any,
	offset, limit int,
) ([]*StoredUser, int, error) {
	baseWhere := `t.type_name = 'User' AND ei.provider_name = $1`
	args := []any{providerName}
	args = append(args, filterArgs...)

	whereClause := baseWhere
	if filterSQL != "" {
		// filterSQL placeholders start at $2 because providerName is $1.
		whereClause = baseWhere + " AND " + shiftPlaceholders(filterSQL, 1)
	}

	var total int
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM entity e
		JOIN entity_type_definition t ON t.id = e.type_id
		JOIN entity_external_identity ei ON ei.entity_id = e.id
		WHERE %s`, whereClause)
	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	pageArgs := make([]any, len(args), len(args)+2)
	copy(pageArgs, args)
	pageArgs = append(pageArgs, limit, offset)
	listSQL := fmt.Sprintf(`
		SELECT e.id, e.properties, ei.external_subject, e.created_at, e.updated_at
		FROM entity e
		JOIN entity_type_definition t ON t.id = e.type_id
		JOIN entity_external_identity ei ON ei.entity_id = e.id
		WHERE %s
		ORDER BY e.created_at, e.id
		LIMIT $%d OFFSET $%d`,
		whereClause, len(args)+1, len(args)+2)

	rows, err := q.Query(ctx, listSQL, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var results []*StoredUser
	for rows.Next() {
		stored := &StoredUser{}
		var propsJSON []byte
		if err := rows.Scan(&stored.EntityID, &propsJSON, &stored.ExternalSubject, &stored.CreatedAt, &stored.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user row: %w", err)
		}
		if err := json.Unmarshal(propsJSON, &stored.Properties); err != nil {
			return nil, 0, fmt.Errorf("unmarshal user properties: %w", err)
		}
		results = append(results, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user rows: %w", err)
	}
	return results, total, nil
}

// Replace overwrites the User's properties. Returns ErrNotFound when the
// caller's provider has no link to the user (or the user does not exist).
// The system_id type-check trigger does not fire because system_id is not
// in the SET clause.
func (r *pgxRepository) Replace(
	ctx context.Context, tx pgx.Tx,
	providerName string, id uuid.UUID, props map[string]any,
) (time.Time, error) {
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal user properties: %w", err)
	}

	var updatedAt time.Time
	err = tx.QueryRow(ctx,
		`UPDATE entity
		 SET properties = $1::jsonb
		 WHERE id = $2
		   AND type_id = (SELECT id FROM entity_type_definition WHERE type_name = 'User')
		   AND EXISTS (
		       SELECT 1 FROM entity_external_identity
		       WHERE entity_id = $2 AND provider_name = $3
		   )
		 RETURNING updated_at`,
		propsJSON, id, providerName,
	).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, fmt.Errorf("update user entity: %w", err)
	}
	return updatedAt, nil
}

// shiftPlaceholders rewrites $N references in a SQL fragment by adding
// `delta` to each N. Used to splice a filter fragment (whose placeholders
// start at $1) into a query that already consumes some placeholders.
func shiftPlaceholders(sql string, delta int) string {
	var sb []byte
	i := 0
	for i < len(sql) {
		c := sql[i]
		if c != '$' {
			sb = append(sb, c)
			i++
			continue
		}
		j := i + 1
		for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
			j++
		}
		if j == i+1 {
			sb = append(sb, c)
			i++
			continue
		}
		n := 0
		for k := i + 1; k < j; k++ {
			n = n*10 + int(sql[k]-'0')
		}
		sb = append(sb, fmt.Sprintf("$%d", n+delta)...)
		i = j
	}
	return string(sb)
}
