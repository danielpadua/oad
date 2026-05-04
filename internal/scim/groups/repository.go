package groups

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

// Repository is the persistence boundary for SCIM Groups. Each Group maps
// to (entity, entity_external_identity, []relation) — the relation rows
// of type 'member_of' are the source of truth for membership.
type Repository interface {
	// Create inserts the group entity, its external identity link, and one
	// member_of relation per memberID. memberIDs must already be validated
	// (existing, owned by the same provider, type User|Group). Returns the
	// stored group with its members projection populated.
	Create(
		ctx context.Context, tx pgx.Tx,
		providerName, externalSubject string,
		props map[string]any, memberIDs []uuid.UUID,
	) (*StoredGroup, error)

	// GetByEntityID fetches the group plus its members projection within
	// the calling provider's tenant. Returns ErrNotFound when the group
	// does not exist or belongs to another provider.
	GetByEntityID(ctx context.Context, q db.DBTX, providerName string, id uuid.UUID) (*StoredGroup, error)

	// Delete removes the calling provider's link to the group. If the
	// group has no remaining provider links, the entity is removed (its
	// member_of relations cascade away). Returns ErrNotFound when the
	// caller has no claim on the group, or ErrBuiltinGroup when the row
	// is is_builtin=true.
	Delete(ctx context.Context, tx pgx.Tx, providerName string, id uuid.UUID) error

	// ValidateMembers returns the subset of candidateIDs that exist as
	// User or Group entities owned by providerName. Caller compares the
	// returned slice length against candidateIDs to detect invalid refs.
	ValidateMembers(ctx context.Context, q db.DBTX, providerName string, candidateIDs []uuid.UUID) ([]uuid.UUID, error)

	// List returns a page of groups belonging to the calling provider
	// together with the total count for pagination. filterSQL is empty
	// when no filter is requested; otherwise it is a WHERE-fragment
	// produced by FilterToSQL with placeholders starting at $1. Members
	// are NOT populated by List — fetching them per row would amount to
	// N+1 and SCIM clients typically use ?attributes to opt out anyway.
	List(
		ctx context.Context, q db.DBTX,
		providerName, filterSQL string, filterArgs []any,
		offset, limit int,
	) (results []*StoredGroup, total int, err error)

	// Replace overwrites the group's stored properties and reconciles its
	// member_of relations against newMemberIDs. Returns ErrNotFound when
	// the caller's provider has no claim on the group; ErrBuiltinGroup
	// when the entity is is_builtin=true. The returned StoredGroup is
	// populated with the resolved member projection.
	Replace(
		ctx context.Context, tx pgx.Tx,
		providerName string, id uuid.UUID,
		props map[string]any, newMemberIDs []uuid.UUID,
	) (*StoredGroup, error)
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

// makeEntityExternalID composes the entity.external_id value for a SCIM-
// provisioned group. Format mirrors the User repository for symmetry:
// "scim:<provider>:<scim-external-id>".
func makeEntityExternalID(providerName, externalSubject string) string {
	return fmt.Sprintf("scim:%s:%s", providerName, externalSubject)
}

func (r *pgxRepository) Create(
	ctx context.Context, tx pgx.Tx,
	providerName, externalSubject string,
	props map[string]any, memberIDs []uuid.UUID,
) (*StoredGroup, error) {
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal group properties: %w", err)
	}

	stored := &StoredGroup{
		Properties:      props,
		ExternalSubject: externalSubject,
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO entity (type_id, external_id, properties)
		 SELECT id, $1, $2::jsonb FROM entity_type_definition WHERE type_name = 'Group'
		 RETURNING id, created_at, updated_at`,
		makeEntityExternalID(providerName, externalSubject), propsJSON,
	).Scan(&stored.EntityID, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("built-in Group type not seeded: %w", err)
		}
		return nil, fmt.Errorf("insert group entity: %w", err)
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

	for _, mid := range memberIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO relation (subject_entity_id, relation_type, target_entity_id)
			 VALUES ($1, 'member_of', $2)`,
			mid, stored.EntityID,
		); err != nil {
			return nil, fmt.Errorf("insert member relation: %w", err)
		}
	}

	if len(memberIDs) > 0 {
		members, err := r.fetchMembers(ctx, tx, stored.EntityID)
		if err != nil {
			return nil, err
		}
		stored.Members = members
	}

	return stored, nil
}

func (r *pgxRepository) GetByEntityID(ctx context.Context, q db.DBTX, providerName string, id uuid.UUID) (*StoredGroup, error) {
	var propsJSON []byte
	stored := &StoredGroup{EntityID: id}

	err := q.QueryRow(ctx,
		`SELECT e.properties, ei.external_subject, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition t ON t.id = e.type_id
		 JOIN entity_external_identity ei ON ei.entity_id = e.id
		 WHERE e.id = $1
		   AND t.type_name = 'Group'
		   AND ei.provider_name = $2`,
		id, providerName,
	).Scan(&propsJSON, &stored.ExternalSubject, &stored.CreatedAt, &stored.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query group: %w", err)
	}

	if err := json.Unmarshal(propsJSON, &stored.Properties); err != nil {
		return nil, fmt.Errorf("unmarshal group properties: %w", err)
	}

	members, err := r.fetchMembers(ctx, q, id)
	if err != nil {
		return nil, err
	}
	stored.Members = members
	return stored, nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, providerName string, id uuid.UUID) error {
	// Reject built-in groups before touching anything. The
	// entity_external_identity check downstream would already filter them
	// out (built-ins have no ei row), but a clear error beats a bare
	// 404 when an unintended target is hit.
	var isBuiltin bool
	err := tx.QueryRow(ctx,
		`SELECT is_builtin FROM entity
		 WHERE id = $1
		   AND type_id = (SELECT id FROM entity_type_definition WHERE type_name = 'Group')`,
		id,
	).Scan(&isBuiltin)
	if err == nil && isBuiltin {
		return ErrBuiltinGroup
	}
	// pgx.ErrNoRows here means the entity is not a Group; downstream
	// queries will correctly report ErrNotFound for the calling provider.

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

	if _, err := tx.Exec(ctx,
		`DELETE FROM entity
		 WHERE id = $1
		   AND NOT EXISTS (SELECT 1 FROM entity_external_identity WHERE entity_id = $1)`,
		id,
	); err != nil {
		return fmt.Errorf("delete group entity: %w", err)
	}
	return nil
}

func (r *pgxRepository) ValidateMembers(
	ctx context.Context, q db.DBTX,
	providerName string, candidateIDs []uuid.UUID,
) ([]uuid.UUID, error) {
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	rows, err := q.Query(ctx,
		`SELECT DISTINCT e.id
		 FROM entity e
		 JOIN entity_type_definition t ON t.id = e.type_id
		 JOIN entity_external_identity ei ON ei.entity_id = e.id
		 WHERE e.id = ANY($1::uuid[])
		   AND t.type_name IN ('User', 'Group')
		   AND ei.provider_name = $2`,
		candidateIDs, providerName,
	)
	if err != nil {
		return nil, fmt.Errorf("validate members: %w", err)
	}
	defer rows.Close()

	resolved := make([]uuid.UUID, 0, len(candidateIDs))
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member id: %w", err)
		}
		resolved = append(resolved, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate member rows: %w", err)
	}
	return resolved, nil
}

func (r *pgxRepository) List(
	ctx context.Context, q db.DBTX,
	providerName, filterSQL string, filterArgs []any,
	offset, limit int,
) ([]*StoredGroup, int, error) {
	baseWhere := `t.type_name = 'Group' AND ei.provider_name = $1`
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
		return nil, 0, fmt.Errorf("count groups: %w", err)
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
		return nil, 0, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var results []*StoredGroup
	for rows.Next() {
		stored := &StoredGroup{}
		var propsJSON []byte
		if err := rows.Scan(&stored.EntityID, &propsJSON, &stored.ExternalSubject, &stored.CreatedAt, &stored.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan group row: %w", err)
		}
		if err := json.Unmarshal(propsJSON, &stored.Properties); err != nil {
			return nil, 0, fmt.Errorf("unmarshal group properties: %w", err)
		}
		results = append(results, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate group rows: %w", err)
	}
	return results, total, nil
}

func (r *pgxRepository) Replace(
	ctx context.Context, tx pgx.Tx,
	providerName string, id uuid.UUID,
	props map[string]any, newMemberIDs []uuid.UUID,
) (*StoredGroup, error) {
	// Reject built-in groups before any mutation. The provider scope
	// downstream would already filter them out, but a clear error beats
	// a silent 404.
	var isBuiltin bool
	err := tx.QueryRow(ctx,
		`SELECT is_builtin FROM entity
		 WHERE id = $1
		   AND type_id = (SELECT id FROM entity_type_definition WHERE type_name = 'Group')`,
		id,
	).Scan(&isBuiltin)
	if err == nil && isBuiltin {
		return nil, ErrBuiltinGroup
	}

	propsJSON, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal group properties: %w", err)
	}

	stored := &StoredGroup{
		EntityID:   id,
		Properties: props,
	}
	var updatedAt time.Time
	err = tx.QueryRow(ctx,
		`UPDATE entity
		 SET properties = $1::jsonb
		 WHERE id = $2
		   AND type_id = (SELECT id FROM entity_type_definition WHERE type_name = 'Group')
		   AND EXISTS (
		       SELECT 1 FROM entity_external_identity
		       WHERE entity_id = $2 AND provider_name = $3
		   )
		 RETURNING updated_at`,
		propsJSON, id, providerName,
	).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update group entity: %w", err)
	}
	stored.UpdatedAt = updatedAt

	if err := tx.QueryRow(ctx,
		`SELECT e.created_at, ei.external_subject
		 FROM entity e
		 JOIN entity_external_identity ei ON ei.entity_id = e.id
		 WHERE e.id = $1 AND ei.provider_name = $2`,
		id, providerName,
	).Scan(&stored.CreatedAt, &stored.ExternalSubject); err != nil {
		return nil, fmt.Errorf("read group metadata after replace: %w", err)
	}

	if err := r.reconcileMembers(ctx, tx, id, newMemberIDs); err != nil {
		return nil, err
	}

	members, err := r.fetchMembers(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	stored.Members = members
	return stored, nil
}

// reconcileMembers diffs the existing member_of relations targeting groupID
// against newMemberIDs and applies the minimal set of inserts and deletes.
// Unchanged relations keep their created_at, preserving deterministic
// ordering for clients that rely on it.
func (r *pgxRepository) reconcileMembers(
	ctx context.Context, tx pgx.Tx,
	groupID uuid.UUID, newMemberIDs []uuid.UUID,
) error {
	rows, err := tx.Query(ctx,
		`SELECT subject_entity_id FROM relation
		 WHERE target_entity_id = $1
		   AND relation_type = 'member_of'
		   AND system_id IS NULL`,
		groupID,
	)
	if err != nil {
		return fmt.Errorf("read existing members: %w", err)
	}
	current := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var mid uuid.UUID
		if err := rows.Scan(&mid); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing member: %w", err)
		}
		current[mid] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate existing members: %w", err)
	}
	rows.Close()

	wanted := make(map[uuid.UUID]struct{}, len(newMemberIDs))
	for _, mid := range newMemberIDs {
		wanted[mid] = struct{}{}
	}

	for mid := range current {
		if _, keep := wanted[mid]; keep {
			continue
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM relation
			 WHERE subject_entity_id = $1
			   AND target_entity_id = $2
			   AND relation_type = 'member_of'
			   AND system_id IS NULL`,
			mid, groupID,
		); err != nil {
			return fmt.Errorf("delete obsolete member relation: %w", err)
		}
	}

	for _, mid := range newMemberIDs {
		if _, exists := current[mid]; exists {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO relation (subject_entity_id, relation_type, target_entity_id)
			 VALUES ($1, 'member_of', $2)`,
			mid, groupID,
		); err != nil {
			return fmt.Errorf("insert new member relation: %w", err)
		}
	}
	return nil
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

// fetchMembers returns the members projection for a group, ordered by
// when each member_of relation was created (stable for clients that
// depend on insertion order).
func (r *pgxRepository) fetchMembers(ctx context.Context, q db.DBTX, groupID uuid.UUID) ([]MemberEntity, error) {
	rows, err := q.Query(ctx,
		`SELECT e.id, t.type_name, COALESCE(e.properties->>'displayName', '')
		 FROM relation r
		 JOIN entity e ON e.id = r.subject_entity_id
		 JOIN entity_type_definition t ON t.id = e.type_id
		 WHERE r.target_entity_id = $1
		   AND r.relation_type = 'member_of'
		   AND r.system_id IS NULL
		 ORDER BY r.created_at, e.id`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch members: %w", err)
	}
	defer rows.Close()

	var members []MemberEntity
	for rows.Next() {
		var m MemberEntity
		if err := rows.Scan(&m.EntityID, &m.Type, &m.Display); err != nil {
			return nil, fmt.Errorf("scan member row: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return members, nil
}
