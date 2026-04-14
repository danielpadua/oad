package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/validation"
)

// Service orchestrates business logic for entity type definitions.
// Every mutation runs inside a transaction that also writes the audit record,
// guaranteeing that no change occurs without a corresponding audit entry (NFR-AUD-001).
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new entity type definition service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create validates and persists a new entity type definition (FR-ETD-001, FR-ETD-004).
func (s *Service) Create(ctx context.Context, req CreateRequest) (*EntityTypeDefinition, error) {
	if apiErr := validateCreate(req); apiErr != nil {
		return nil, apiErr
	}

	etd := &EntityTypeDefinition{
		TypeName:          req.TypeName,
		AllowedProperties: req.AllowedProperties,
		AllowedRelations:  req.AllowedRelations,
		Scope:             req.Scope,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, etd); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(etd)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "entity_type_definition",
			ResourceID:   etd.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return etd, nil
}

// GetByID retrieves a single entity type definition by its UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*EntityTypeDefinition, error) {
	etd, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return etd, nil
}

// List returns all entity type definitions ordered by type_name.
func (s *Service) List(ctx context.Context) ([]*EntityTypeDefinition, error) {
	items, err := s.repo.List(ctx, s.pool)
	if err != nil {
		return nil, fmt.Errorf("listing entity type definitions: %w", err)
	}
	if items == nil {
		items = []*EntityTypeDefinition{}
	}
	return items, nil
}

// Update replaces the mutable fields of an entity type definition (FR-ETD-002, FR-ETD-004).
// TypeName and Scope are immutable after creation.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*EntityTypeDefinition, error) {
	if apiErr := validateUpdate(req); apiErr != nil {
		return nil, apiErr
	}

	var result *EntityTypeDefinition
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		beforeJSON, _ := json.Marshal(existing)

		existing.AllowedProperties = req.AllowedProperties
		existing.AllowedRelations = req.AllowedRelations

		if err := s.repo.Update(ctx, tx, existing); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(existing)
		result = existing
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpUpdate,
			ResourceType: "entity_type_definition",
			ResourceID:   existing.ID,
			BeforeValue:  beforeJSON,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return result, nil
}

// Delete removes an entity type definition (FR-ETD-003).
// Returns an error when entities of this type still exist.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Pre-check outside the transaction to return a descriptive error early
	// (the FK constraint provides defense-in-depth inside the transaction).
	hasEntities, err := s.repo.HasEntities(ctx, s.pool, id)
	if err != nil {
		return fmt.Errorf("checking entities for type %s: %w", id, err)
	}
	if hasEntities {
		return apierr.BadRequest("entity type definition has associated entities and cannot be deleted")
	}

	err = db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		beforeJSON, _ := json.Marshal(existing)

		if err := s.repo.Delete(ctx, tx, id); err != nil {
			return err
		}
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpDelete,
			ResourceType: "entity_type_definition",
			ResourceID:   id,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
}

// validateCreate checks all required fields and validates JSON Schema content.
func validateCreate(req CreateRequest) *apierr.APIError {
	if req.TypeName == "" {
		return apierr.BadRequest("type_name is required")
	}
	if req.Scope != ScopeGlobal && req.Scope != ScopeSystemScoped {
		return apierr.BadRequest("scope must be 'global' or 'system_scoped'")
	}
	if len(req.AllowedProperties) == 0 {
		return apierr.BadRequest("allowed_properties is required")
	}
	if len(req.AllowedRelations) == 0 {
		return apierr.BadRequest("allowed_relations is required")
	}
	if apiErr := validation.ValidateIsJSONSchema(req.AllowedProperties); apiErr != nil {
		return apierr.ValidationFailed(
			append([]string{"allowed_properties: invalid JSON Schema"}, apiErr.Details...)...,
		)
	}
	return validateRelationsJSON(req.AllowedRelations)
}

// validateUpdate checks that the replacement fields are non-empty and valid.
func validateUpdate(req UpdateRequest) *apierr.APIError {
	if len(req.AllowedProperties) == 0 {
		return apierr.BadRequest("allowed_properties is required")
	}
	if len(req.AllowedRelations) == 0 {
		return apierr.BadRequest("allowed_relations is required")
	}
	if apiErr := validation.ValidateIsJSONSchema(req.AllowedProperties); apiErr != nil {
		return apierr.ValidationFailed(
			append([]string{"allowed_properties: invalid JSON Schema"}, apiErr.Details...)...,
		)
	}
	return validateRelationsJSON(req.AllowedRelations)
}

// validateRelationsJSON ensures allowed_relations is a JSON object mapping
// relation names to configuration objects ({"member": {"target_types": [...]}}).
func validateRelationsJSON(data json.RawMessage) *apierr.APIError {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return apierr.BadRequest("allowed_relations must be a JSON object: " + err.Error())
	}
	return nil
}

// toAPIError converts domain and repository errors to *apierr.APIError.
// Errors already typed as *apierr.APIError pass through unchanged.
// Unrecognised errors are returned as-is so handlers can log and map them to 500.
func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *apierr.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apierr.NotFound("entity type definition")
	case errors.Is(err, ErrDuplicateTypeName):
		return apierr.Conflict("entity type name already exists")
	case errors.Is(err, ErrHasEntities):
		return apierr.BadRequest("entity type definition has associated entities and cannot be deleted")
	}
	return err
}
