package useradmin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/entity"
)

// Repository abstracts the persistence layer for admin user queries.
type Repository interface {
	List(ctx context.Context, q db.DBTX, params ListParams) (*ListResult, error)
}

type pgxRepository struct{}

// NewRepository creates a new PostgreSQL-backed repository for user admin.
func NewRepository() Repository {
	return &pgxRepository{}
}

func (r *pgxRepository) List(ctx context.Context, q db.DBTX, params ListParams) (*ListResult, error) {
	// First query: count total users
	var total int64
	countQuery := `
		SELECT COUNT(*)
		FROM entity e
		JOIN entity_type_definition etd ON etd.id = e.type_id
		WHERE etd.type_name = 'User'
	`
	if err := q.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting users: %w", err)
	}

	if total == 0 {
		return &ListResult{
			Items:  []*User{},
			Total:  0,
			Limit:  params.Limit,
			Offset: params.Offset,
		}, nil
	}

	// Second query: fetch paginated entities
	listQuery := `
		SELECT e.id, e.type_id, etd.type_name, e.external_id, e.properties, e.system_id, e.created_at, e.updated_at
		FROM entity e
		JOIN entity_type_definition etd ON etd.id = e.type_id
		WHERE etd.type_name = 'User'
		ORDER BY e.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := q.Query(ctx, listQuery, params.Limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}

	var users []*User
	var entityIDs []uuid.UUID
	usersByID := make(map[uuid.UUID]*User)

	for rows.Next() {
		e := &entity.Entity{}
		if err := rows.Scan(
			&e.ID, &e.TypeID, &e.Type, &e.ExternalID,
			&e.Properties, &e.SystemID, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning user entity: %w", err)
		}

		u := &User{
			Entity:             e,
			ExternalIdentities: []ExternalIdentity{}, // Initialize empty to avoid null in JSON
		}
		users = append(users, u)
		entityIDs = append(entityIDs, e.ID)
		usersByID[e.ID] = u
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}

	if len(entityIDs) > 0 {
		// Third query: fetch external identities for the fetched entityIDs
		identitiesQuery := `
			SELECT entity_id, provider_name, external_subject
			FROM entity_external_identity
			WHERE entity_id = ANY($1)
		`
		identRows, err := q.Query(ctx, identitiesQuery, entityIDs)
		if err != nil {
			return nil, fmt.Errorf("querying external identities: %w", err)
		}

		for identRows.Next() {
			var entityID uuid.UUID
			var ei ExternalIdentity
			if err := identRows.Scan(&entityID, &ei.ProviderName, &ei.ExternalSubject); err != nil {
				identRows.Close()
				return nil, fmt.Errorf("scanning external identity: %w", err)
			}
			if u, ok := usersByID[entityID]; ok {
				u.ExternalIdentities = append(u.ExternalIdentities, ei)
			}
		}
		identRows.Close()
		if err := identRows.Err(); err != nil {
			return nil, fmt.Errorf("iterating external identities: %w", err)
		}
	}

	return &ListResult{
		Items:  users,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}
