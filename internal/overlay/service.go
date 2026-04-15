package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/auth"
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/validation"
)

// overlaySchema holds the fields from system_overlay_schema needed for validation.
type overlaySchema struct {
	ID                       uuid.UUID
	AllowedOverlayProperties json.RawMessage
}

// entityMeta holds the fields from entity and system needed for overlay validation.
type entityMeta struct {
	TypeID uuid.UUID
}

// Service orchestrates business logic for property overlays.
// All write operations run inside a transaction that also writes the audit record.
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new property overlay service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create attaches overlay properties to an entity for the caller's system scope (FR-OVL-001..004, FR-OVL-008).
// The system_id is always derived from the authenticated identity — callers cannot self-assign a different system.
func (s *Service) Create(ctx context.Context, entityID uuid.UUID, req CreateRequest) (*PropertyOverlay, error) {
	if len(req.Properties) == 0 {
		return nil, apierr.BadRequest("properties is required")
	}

	systemID, systemName, apiErr := s.resolveCallerSystem(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	meta, apiErr := s.resolveEntity(ctx, s.pool, entityID)
	if apiErr != nil {
		return nil, apiErr
	}

	schema, apiErr := s.resolveOverlaySchema(ctx, systemID, meta.TypeID)
	if apiErr != nil {
		return nil, apiErr
	}

	if apiErr := validateOverlayProperties(req.Properties, systemName, schema.AllowedOverlayProperties); apiErr != nil {
		return nil, apiErr
	}

	o := &PropertyOverlay{
		EntityID:   entityID,
		SystemID:   systemID,
		Properties: req.Properties,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, o); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(o)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "property_overlay",
			ResourceID:   o.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return o, nil
}

// GetByID retrieves a single property overlay by ID.
// System-scoped callers may only access overlays belonging to their own system (FR-OVL-008).
// Platform admins (empty SystemID) may access any overlay.
func (s *Service) GetByID(ctx context.Context, entityID, overlayID uuid.UUID) (*PropertyOverlay, error) {
	o, err := s.repo.GetByID(ctx, s.pool, overlayID)
	if err != nil {
		return nil, toAPIError(err)
	}
	if o.EntityID != entityID {
		return nil, apierr.NotFound("property overlay")
	}
	if apiErr := s.assertSystemAccess(ctx, o.SystemID); apiErr != nil {
		return nil, apiErr
	}
	return o, nil
}

// ListByEntity returns a paginated list of property overlays for the given entity.
// System-scoped callers see only their own system's overlay; admins see all.
func (s *Service) ListByEntity(ctx context.Context, entityID uuid.UUID, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	// Verify entity exists before listing overlays.
	if _, apiErr := s.resolveEntity(ctx, s.pool, entityID); apiErr != nil {
		return nil, apiErr
	}

	// System-scoped callers may only see their own overlay: apply an in-memory
	// filter after the paginated query so the count stays consistent with the
	// filtered view.
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID != "" {
		callerSysID, parseErr := uuid.Parse(identity.SystemID)
		if parseErr != nil {
			return nil, apierr.Internal("invalid system_id in token")
		}
		// A system can have at most one overlay per entity (UNIQUE constraint),
		// so the result set is always 0 or 1 items. Fetch without pagination and
		// wrap in the standard result shape.
		all, _, err := s.repo.ListByEntity(ctx, s.pool, entityID, ListParams{Limit: 2, Offset: 0})
		if err != nil {
			return nil, fmt.Errorf("listing property overlays: %w", err)
		}
		filtered := make([]*PropertyOverlay, 0, 1)
		for _, o := range all {
			if o.SystemID == callerSysID {
				filtered = append(filtered, o)
			}
		}
		return &ListResult{
			Items:  filtered,
			Total:  int64(len(filtered)),
			Limit:  params.Limit,
			Offset: params.Offset,
		}, nil
	}

	items, total, err := s.repo.ListByEntity(ctx, s.pool, entityID, params)
	if err != nil {
		return nil, fmt.Errorf("listing property overlays: %w", err)
	}
	if items == nil {
		items = []*PropertyOverlay{}
	}
	return &ListResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// Update replaces the overlay properties for an existing overlay (FR-OVL-001..004, FR-OVL-008).
// Only the owning system may update its overlay.
func (s *Service) Update(ctx context.Context, entityID, overlayID uuid.UUID, req UpdateRequest) (*PropertyOverlay, error) {
	if len(req.Properties) == 0 {
		return nil, apierr.BadRequest("properties is required")
	}

	systemID, systemName, apiErr := s.resolveCallerSystem(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	meta, apiErr := s.resolveEntity(ctx, s.pool, entityID)
	if apiErr != nil {
		return nil, apiErr
	}

	schema, apiErr := s.resolveOverlaySchema(ctx, systemID, meta.TypeID)
	if apiErr != nil {
		return nil, apiErr
	}

	if apiErr := validateOverlayProperties(req.Properties, systemName, schema.AllowedOverlayProperties); apiErr != nil {
		return nil, apiErr
	}

	var result *PropertyOverlay
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, overlayID)
		if err != nil {
			return err
		}
		if existing.EntityID != entityID {
			return ErrNotFound
		}
		if existing.SystemID != systemID {
			return ErrNotFound
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
			ResourceType: "property_overlay",
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

// Delete removes a property overlay (FR-OVL-008).
// Only the owning system may delete its overlay.
func (s *Service) Delete(ctx context.Context, entityID, overlayID uuid.UUID) error {
	systemID, _, apiErr := s.resolveCallerSystem(ctx)
	if apiErr != nil {
		return apiErr
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, overlayID)
		if err != nil {
			return err
		}
		if existing.EntityID != entityID {
			return ErrNotFound
		}
		if existing.SystemID != systemID {
			return ErrNotFound
		}
		beforeJSON, _ := json.Marshal(existing)

		if err := s.repo.Delete(ctx, tx, overlayID); err != nil {
			return err
		}
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpDelete,
			ResourceType: "property_overlay",
			ResourceID:   overlayID,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
}

// resolveCallerSystem extracts the system UUID and name from the authenticated
// identity. Returns ErrNoSystemScope when the caller is a platform admin
// (no system scope), since platform admins cannot own overlays (FR-OVL-008).
func (s *Service) resolveCallerSystem(ctx context.Context) (uuid.UUID, string, *apierr.APIError) {
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID == "" {
		return uuid.Nil, "", apierr.Forbidden("a system scope is required to manage property overlays")
	}
	systemID, err := uuid.Parse(identity.SystemID)
	if err != nil {
		return uuid.Nil, "", apierr.Internal("invalid system_id in token")
	}

	var name string
	err = s.pool.QueryRow(ctx,
		`SELECT name FROM system WHERE id = $1 AND active = true`, systemID,
	).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", apierr.Forbidden("system not found or inactive")
		}
		return uuid.Nil, "", apierr.Internal("failed to resolve system")
	}
	return systemID, name, nil
}

// resolveEntity retrieves entity metadata required for overlay validation.
func (s *Service) resolveEntity(ctx context.Context, q db.DBTX, entityID uuid.UUID) (*entityMeta, *apierr.APIError) {
	meta := &entityMeta{}
	err := q.QueryRow(ctx,
		`SELECT type_id FROM entity WHERE id = $1`, entityID,
	).Scan(&meta.TypeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierr.NotFound("entity")
		}
		return nil, apierr.Internal("failed to resolve entity")
	}
	return meta, nil
}

// resolveOverlaySchema looks up the system overlay schema for the given
// (system_id, entity_type_id) pair. Returns a 400 error when no schema
// is declared, as required by FR-OVL-003.
func (s *Service) resolveOverlaySchema(ctx context.Context, systemID, entityTypeID uuid.UUID) (*overlaySchema, *apierr.APIError) {
	schema := &overlaySchema{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, allowed_overlay_properties
		 FROM system_overlay_schema
		 WHERE system_id = $1 AND entity_type_id = $2`,
		systemID, entityTypeID,
	).Scan(&schema.ID, &schema.AllowedOverlayProperties)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierr.UnprocessableEntity(
				"no system overlay schema declared for this system and entity type; " +
					"create one at POST /api/v1/systems/{system_id}/overlay-schemas",
			)
		}
		return nil, apierr.Internal("failed to resolve overlay schema")
	}
	return schema, nil
}

// assertSystemAccess checks that the caller's system scope matches the overlay's
// system. Platform admins (empty SystemID) bypass the check.
func (s *Service) assertSystemAccess(ctx context.Context, overlaySystemID uuid.UUID) *apierr.APIError {
	identity := auth.MustIdentityFromContext(ctx)
	if identity.SystemID == "" {
		return nil // platform admin
	}
	callerSysID, err := uuid.Parse(identity.SystemID)
	if err != nil {
		return apierr.Internal("invalid system_id in token")
	}
	if callerSysID != overlaySystemID {
		return apierr.NotFound("property overlay")
	}
	return nil
}

// validateOverlayProperties enforces namespace prefix (FR-OVL-004) and
// validates the properties instance against the overlay schema (FR-OVL-002).
func validateOverlayProperties(props json.RawMessage, systemName string, schemaDoc json.RawMessage) *apierr.APIError {
	if apiErr := validateNamespacePrefix(props, systemName); apiErr != nil {
		return apiErr
	}
	validator, err := validation.Compile("oad://overlay/properties", schemaDoc)
	if err != nil {
		return apierr.Internal("failed to compile overlay schema")
	}
	return validator.ValidateRaw(props)
}

// validateNamespacePrefix checks that every top-level key in the properties
// JSON object carries the required "{systemName}." prefix (FR-OVL-004).
func validateNamespacePrefix(props json.RawMessage, systemName string) *apierr.APIError {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(props, &doc); err != nil {
		return apierr.BadRequest("invalid JSON in properties: " + err.Error())
	}

	prefix := systemName + "."
	var violations []string
	for key := range doc {
		if !strings.HasPrefix(key, prefix) {
			violations = append(violations, "'"+key+"'")
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return apierr.ValidationFailed(fmt.Sprintf(
			"all property keys must be prefixed with %q: %s",
			prefix, strings.Join(violations, ", "),
		))
	}
	return nil
}

// toAPIError maps domain and repository errors to *apierr.APIError.
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
		return apierr.NotFound("property overlay")
	case errors.Is(err, ErrDuplicate):
		return apierr.Conflict("property overlay already exists for this entity and system")
	case errors.Is(err, ErrEntityNotFound):
		return apierr.NotFound("entity")
	case errors.Is(err, ErrNoSchema):
		return apierr.UnprocessableEntity("no system overlay schema declared for this system and entity type")
	case errors.Is(err, ErrNoSystemScope):
		return apierr.Forbidden("a system scope is required to manage property overlays")
	}
	return err
}
