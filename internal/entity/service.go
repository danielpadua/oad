package entity

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
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/validation"
)

// entityTypeDef holds the type definition fields needed for entity validation.
type entityTypeDef struct {
	ID                uuid.UUID
	TypeName          string
	AllowedProperties json.RawMessage
	Scope             string
}

// Service orchestrates business logic for entity management.
// Every mutation runs inside a transaction that also writes the audit record,
// guaranteeing that no mutation occurs without a corresponding audit entry (NFR-AUD-001).
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new entity service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create validates and persists a new entity (FR-ENT-001, FR-ENT-003, FR-ENT-008).
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Entity, error) {
	if apiErr := validateCreateRequest(req); apiErr != nil {
		return nil, apiErr
	}

	typeDef, apiErr := s.resolveType(ctx, s.pool, req.Type)
	if apiErr != nil {
		return nil, apiErr
	}

	effectiveSystemID, apiErr := resolveEffectiveSystemID(ctx, req.SystemID)
	if apiErr != nil {
		return nil, apiErr
	}

	if apiErr := validateScopeConstraints(typeDef, effectiveSystemID); apiErr != nil {
		return nil, apiErr
	}

	props := req.Properties
	if len(props) == 0 {
		props = json.RawMessage(`{}`)
	}

	if apiErr := validateProperties(props, typeDef.AllowedProperties); apiErr != nil {
		return nil, apiErr
	}

	e := &Entity{
		TypeID:     typeDef.ID,
		Type:       typeDef.TypeName,
		ExternalID: req.ExternalID,
		Properties: props,
		SystemID:   effectiveSystemID,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, e); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(e)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "entity",
			ResourceID:   e.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return e, nil
}

// GetByID retrieves a single entity by its UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Entity, error) {
	e, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return e, nil
}

// GetByTypeAndExternalID retrieves an entity by type name + external_id (FR-ENT-004).
func (s *Service) GetByTypeAndExternalID(ctx context.Context, typeName, externalID string) (*Entity, error) {
	typeDef, apiErr := s.resolveType(ctx, s.pool, typeName)
	if apiErr != nil {
		return nil, apiErr
	}
	e, err := s.repo.GetByTypeAndExternalID(ctx, s.pool, typeDef.ID, externalID)
	if err != nil {
		return nil, toAPIError(err)
	}
	return e, nil
}

// List returns a paginated list of entities, optionally filtered by type name.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.List(ctx, s.pool, params)
	if err != nil {
		return nil, fmt.Errorf("listing entities: %w", err)
	}
	return &ListResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// Update replaces an entity's properties (FR-ENT-005).
// Type, external_id, and system_id are immutable after creation.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Entity, error) {
	if len(req.Properties) == 0 {
		return nil, apierr.BadRequest("properties is required")
	}

	var result *Entity
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}

		typeDef, apiErr := s.resolveType(ctx, tx, existing.Type)
		if apiErr != nil {
			return apiErr
		}

		if apiErr := validateProperties(req.Properties, typeDef.AllowedProperties); apiErr != nil {
			return apiErr
		}

		beforeJSON, _ := json.Marshal(existing)
		existing.Properties = req.Properties

		if err := s.repo.Update(ctx, tx, existing); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(existing)
		result = existing
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpUpdate,
			ResourceType: "entity",
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

// Delete removes an entity and, via FK CASCADE, its property overlays and
// relations (FR-ENT-006).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
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
			ResourceType: "entity",
			ResourceID:   id,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
}

// BulkCreate processes a batch of entity requests in a single API call (FR-ENT-007).
// Each item is processed independently; individual failures are collected without
// stopping the batch. In "upsert" mode, existing entities have their properties updated.
func (s *Service) BulkCreate(ctx context.Context, req BulkCreateRequest) (*BulkCreateResult, error) {
	if len(req.Entities) == 0 {
		return nil, apierr.BadRequest("entities list must not be empty")
	}

	mode := req.Mode
	if mode == "" {
		mode = "create"
	}
	if mode != "create" && mode != "upsert" {
		return nil, apierr.BadRequest("mode must be 'create' or 'upsert'")
	}

	result := &BulkCreateResult{
		Total:  len(req.Entities),
		Errors: []BulkItemError{},
	}

	for i, item := range req.Entities {
		if mode == "upsert" {
			created, err := s.createOrUpdate(ctx, item)
			if err != nil {
				result.Errors = append(result.Errors, BulkItemError{Index: i, Error: err.Error()})
				continue
			}
			if created {
				result.Created++
			} else {
				result.Updated++
			}
		} else {
			if _, err := s.Create(ctx, item); err != nil {
				result.Errors = append(result.Errors, BulkItemError{Index: i, Error: err.Error()})
				continue
			}
			result.Created++
		}
	}

	return result, nil
}

// createOrUpdate creates or updates an entity. Returns true if created, false if updated.
func (s *Service) createOrUpdate(ctx context.Context, req CreateRequest) (bool, error) {
	if apiErr := validateCreateRequest(req); apiErr != nil {
		return false, apiErr
	}

	typeDef, apiErr := s.resolveType(ctx, s.pool, req.Type)
	if apiErr != nil {
		return false, apiErr
	}

	effectiveSystemID, apiErr := resolveEffectiveSystemID(ctx, req.SystemID)
	if apiErr != nil {
		return false, apiErr
	}
	req.SystemID = effectiveSystemID

	if apiErr := validateScopeConstraints(typeDef, effectiveSystemID); apiErr != nil {
		return false, apiErr
	}

	props := req.Properties
	if len(props) == 0 {
		props = json.RawMessage(`{}`)
	}

	if apiErr := validateProperties(props, typeDef.AllowedProperties); apiErr != nil {
		return false, apiErr
	}

	existing, err := s.repo.GetByTypeAndExternalID(ctx, s.pool, typeDef.ID, req.ExternalID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return false, fmt.Errorf("checking existing entity: %w", err)
	}

	if existing != nil {
		if _, updateErr := s.Update(ctx, existing.ID, UpdateRequest{Properties: props}); updateErr != nil {
			return false, updateErr
		}
		return false, nil
	}

	if _, createErr := s.Create(ctx, req); createErr != nil {
		return false, createErr
	}
	return true, nil
}

// resolveType looks up an entity type definition by type_name.
// q can be a *pgxpool.Pool or a pgx.Tx, enabling use inside transactions.
func (s *Service) resolveType(ctx context.Context, q db.DBTX, typeName string) (*entityTypeDef, *apierr.APIError) {
	td := &entityTypeDef{}
	err := q.QueryRow(ctx,
		`SELECT id, type_name, allowed_properties, scope
		 FROM entity_type_definition
		 WHERE type_name = $1`,
		typeName,
	).Scan(&td.ID, &td.TypeName, &td.AllowedProperties, &td.Scope)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierr.BadRequest("entity type '" + typeName + "' does not exist")
		}
		return nil, apierr.Internal("failed to resolve entity type")
	}
	return td, nil
}

// validateCreateRequest checks required fields before expensive DB lookups.
func validateCreateRequest(req CreateRequest) *apierr.APIError {
	if req.Type == "" {
		return apierr.BadRequest("type is required")
	}
	if req.ExternalID == "" {
		return apierr.BadRequest("external_id is required")
	}
	return nil
}

// resolveEffectiveSystemID reconciles the request's system_id with the caller's
// identity. System-scoped callers have their system_id derived from the token
// (the body field is optional); they may not target a different system.
// Platform admins pass through whatever the body specifies.
func resolveEffectiveSystemID(ctx context.Context, reqSystemID *uuid.UUID) (*uuid.UUID, *apierr.APIError) {
	identity, ok := auth.IdentityFromContext(ctx)
	if !ok || identity.SystemID == "" {
		return reqSystemID, nil
	}
	callerSys, err := uuid.Parse(identity.SystemID)
	if err != nil {
		return nil, apierr.Unauthorized("invalid system scope in identity")
	}
	if reqSystemID != nil && *reqSystemID != callerSys {
		return nil, apierr.Forbidden("cannot target a system outside caller scope")
	}
	return &callerSys, nil
}

// validateScopeConstraints enforces system_id rules based on entity type scope.
// System-scoped types require a system_id; global types must not have one.
func validateScopeConstraints(typeDef *entityTypeDef, systemID *uuid.UUID) *apierr.APIError {
	if typeDef.Scope == "system_scoped" && systemID == nil {
		return apierr.BadRequest("system_id is required for system-scoped entity type '" + typeDef.TypeName + "'")
	}
	if typeDef.Scope == "global" && systemID != nil {
		return apierr.BadRequest("system_id must not be set for global entity type '" + typeDef.TypeName + "'")
	}
	return nil
}

// validateProperties compiles the type's JSON Schema and validates the properties
// instance against it (FR-ENT-003).
func validateProperties(props, schemaDoc json.RawMessage) *apierr.APIError {
	validator, err := validation.Compile("oad://entity/properties", schemaDoc)
	if err != nil {
		return apierr.Internal("failed to compile property schema")
	}
	return validator.ValidateRaw(props)
}

// toAPIError converts domain and repository errors to *apierr.APIError.
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
		return apierr.NotFound("entity")
	case errors.Is(err, ErrDuplicateExternalID):
		return apierr.Conflict("entity with this type and external_id already exists")
	}
	return err
}
