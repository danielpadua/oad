package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors returned by IdentityResolver.Resolve.
var (
	ErrNotProvisioned = errors.New("user not provisioned: no entity linked to this provider/subject")
	ErrDisabled       = errors.New("user is disabled (entity.properties.active = false)")
)

// ResolverRepository is the persistence boundary for identity resolution.
// All methods use the pool directly (read-only, no transaction needed).
type ResolverRepository interface {
	// FindEntityID resolves (provider, sub) → entity.id via entity_external_identity.
	// Returns ErrNotProvisioned when no row exists.
	FindEntityID(ctx context.Context, providerName, externalSubject string) (uuid.UUID, error)
	// IsActive returns entity.properties->>'active' as bool.
	// Missing property is treated as true (backward compatibility per design §13).
	IsActive(ctx context.Context, entityID uuid.UUID) (bool, error)
	// Groups returns the external_id of all Groups the user is member_of (one hop).
	Groups(ctx context.Context, entityID uuid.UUID) ([]string, error)
	// AllowedSystems returns entity.id of all Systems accessible by the user
	// directly or via any of the given group external IDs.
	AllowedSystems(ctx context.Context, entityID uuid.UUID, groupExternalIDs []string) ([]uuid.UUID, error)
}

// IdentityResolver resolves a (provider, sub) pair into a full Identity
// by querying the entity/relation graph. It holds no state across calls;
// caching is handled by the caller (see IdentityCache in cache.go).
type IdentityResolver struct {
	repo ResolverRepository
}

// NewIdentityResolver creates a resolver backed by the given repository.
func NewIdentityResolver(repo ResolverRepository) *IdentityResolver {
	return &IdentityResolver{repo: repo}
}

// Resolve builds an Identity from the entity/relation graph.
// pool is accepted for consistency with callers that may pass nil during tests
// using a stub repo. The actual pool is only used by pgxResolverRepository.
func (r *IdentityResolver) Resolve(ctx context.Context, _ *pgxpool.Pool, provider, sub string) (*Identity, error) {
	entityID, err := r.repo.FindEntityID(ctx, provider, sub)
	if err != nil {
		return nil, err // ErrNotProvisioned propagates as-is
	}

	active, err := r.repo.IsActive(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("checking active flag: %w", err)
	}
	if !active {
		return nil, ErrDisabled
	}

	groups, err := r.repo.Groups(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("resolving groups: %w", err)
	}

	allowedSystems, err := r.repo.AllowedSystems(ctx, entityID, groups)
	if err != nil {
		return nil, fmt.Errorf("resolving allowed systems: %w", err)
	}

	id := &Identity{
		Subject:        sub,
		Provider:       provider,
		EntityID:       entityID,
		Groups:         groups,
		AllowedSystems: allowedSystems,
		AuthMode:       "jwt",
	}
	for _, g := range groups {
		if g == "oad:admin" {
			id.IsPlatformAdmin = true
			break
		}
	}

	return id, nil
}

// pgxResolverRepository is the production PostgreSQL-backed implementation.
type pgxResolverRepository struct {
	pool *pgxpool.Pool
}

// NewResolverRepository returns the default PostgreSQL-backed repository.
func NewResolverRepository(pool *pgxpool.Pool) ResolverRepository {
	return &pgxResolverRepository{pool: pool}
}

func (r *pgxResolverRepository) FindEntityID(ctx context.Context, providerName, externalSubject string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT entity_id FROM entity_external_identity
		 WHERE provider_name = $1 AND external_subject = $2`,
		providerName, externalSubject,
	).Scan(&id)
	if err != nil {
		// pgx.ErrNoRows means the user has not been SCIM-provisioned yet.
		return uuid.UUID{}, ErrNotProvisioned
	}
	return id, nil
}

func (r *pgxResolverRepository) IsActive(ctx context.Context, entityID uuid.UUID) (bool, error) {
	var raw *bool
	err := r.pool.QueryRow(ctx,
		`SELECT NULLIF(properties->>'active', '')::boolean FROM entity WHERE id = $1`,
		entityID,
	).Scan(&raw)
	if err != nil {
		return false, fmt.Errorf("querying entity active flag: %w", err)
	}
	if raw == nil {
		return true, nil // missing property treated as active per design §13
	}
	return *raw, nil
}

func (r *pgxResolverRepository) Groups(ctx context.Context, entityID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.external_id
		 FROM relation rel
		 JOIN entity e ON e.id = rel.target_id
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE rel.subject_id = $1
		   AND rel.relation_type = 'member_of'
		   AND etd.type_name = 'Group'`,
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying group memberships: %w", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var externalID string
		if err := rows.Scan(&externalID); err != nil {
			return nil, fmt.Errorf("scanning group external_id: %w", err)
		}
		groups = append(groups, externalID)
	}
	return groups, rows.Err()
}

func (r *pgxResolverRepository) AllowedSystems(ctx context.Context, entityID uuid.UUID, groupExternalIDs []string) ([]uuid.UUID, error) {
	// Resolve group entity IDs from their external_ids.
	var groupIDs []uuid.UUID
	if len(groupExternalIDs) > 0 {
		rows, err := r.pool.Query(ctx,
			`SELECT id FROM entity WHERE external_id = ANY($1)`,
			groupExternalIDs,
		)
		if err != nil {
			return nil, fmt.Errorf("resolving group entity IDs: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var gid uuid.UUID
			if err := rows.Scan(&gid); err != nil {
				return nil, fmt.Errorf("scanning group id: %w", err)
			}
			groupIDs = append(groupIDs, gid)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Build the subject ID set: user + all groups.
	subjectIDs := make([]uuid.UUID, 0, 1+len(groupIDs))
	subjectIDs = append(subjectIDs, entityID)
	subjectIDs = append(subjectIDs, groupIDs...)

	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT rel.target_id
		 FROM relation rel
		 JOIN entity e ON e.id = rel.target_id
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE rel.relation_type = 'has_role_in'
		   AND etd.type_name = 'System'
		   AND rel.subject_id = ANY($1)`,
		subjectIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying allowed systems: %w", err)
	}
	defer rows.Close()

	var systems []uuid.UUID
	for rows.Next() {
		var sid uuid.UUID
		if err := rows.Scan(&sid); err != nil {
			return nil, fmt.Errorf("scanning system id: %w", err)
		}
		systems = append(systems, sid)
	}
	return systems, rows.Err()
}
