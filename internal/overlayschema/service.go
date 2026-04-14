package overlayschema

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
	"github.com/danielpadua/oad/internal/db"
	"github.com/danielpadua/oad/internal/validation"
)

// Service orchestrates business logic for system overlay schemas.
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new system overlay schema service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create validates and persists a new system overlay schema (FR-OVS-001, FR-OVS-004, FR-OVS-005).
func (s *Service) Create(ctx context.Context, systemID uuid.UUID, req CreateRequest) (*SystemOverlaySchema, error) {
	if req.EntityTypeID == uuid.Nil {
		return nil, apierr.BadRequest("entity_type_id is required")
	}
	if len(req.AllowedOverlayProperties) == 0 {
		return nil, apierr.BadRequest("allowed_overlay_properties is required")
	}

	systemName, apiErr := s.resolveSystemName(ctx, systemID)
	if apiErr != nil {
		return nil, apiErr
	}

	if apiErr := validateSchema(req.AllowedOverlayProperties, systemName); apiErr != nil {
		return nil, apiErr
	}

	schema := &SystemOverlaySchema{
		SystemID:                 systemID,
		EntityTypeID:             req.EntityTypeID,
		AllowedOverlayProperties: req.AllowedOverlayProperties,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, schema); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(schema)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "system_overlay_schema",
			ResourceID:   schema.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return schema, nil
}

// GetByID retrieves a single system overlay schema, verifying it belongs to the given system.
func (s *Service) GetByID(ctx context.Context, systemID, schemaID uuid.UUID) (*SystemOverlaySchema, error) {
	schema, err := s.repo.GetByID(ctx, s.pool, schemaID)
	if err != nil {
		return nil, toAPIError(err)
	}
	if schema.SystemID != systemID {
		return nil, apierr.NotFound("system overlay schema")
	}
	return schema, nil
}

// ListBySystem returns all overlay schemas for the given system.
func (s *Service) ListBySystem(ctx context.Context, systemID uuid.UUID) ([]*SystemOverlaySchema, error) {
	items, err := s.repo.ListBySystem(ctx, s.pool, systemID)
	if err != nil {
		return nil, fmt.Errorf("listing system overlay schemas: %w", err)
	}
	if items == nil {
		items = []*SystemOverlaySchema{}
	}
	return items, nil
}

// Update replaces the allowed_overlay_properties of an existing schema (FR-OVS-002, FR-OVS-004, FR-OVS-005).
func (s *Service) Update(ctx context.Context, systemID, schemaID uuid.UUID, req UpdateRequest) (*SystemOverlaySchema, error) {
	if len(req.AllowedOverlayProperties) == 0 {
		return nil, apierr.BadRequest("allowed_overlay_properties is required")
	}

	systemName, apiErr := s.resolveSystemName(ctx, systemID)
	if apiErr != nil {
		return nil, apiErr
	}

	if apiErr := validateSchema(req.AllowedOverlayProperties, systemName); apiErr != nil {
		return nil, apiErr
	}

	var result *SystemOverlaySchema
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, schemaID)
		if err != nil {
			return err
		}
		if existing.SystemID != systemID {
			return ErrNotFound
		}
		beforeJSON, _ := json.Marshal(existing)

		existing.AllowedOverlayProperties = req.AllowedOverlayProperties
		if err := s.repo.Update(ctx, tx, existing); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(existing)
		result = existing
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpUpdate,
			ResourceType: "system_overlay_schema",
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

// Delete removes a system overlay schema (FR-OVS-003).
// Fails if property overlays validated by this schema still exist.
func (s *Service) Delete(ctx context.Context, systemID, schemaID uuid.UUID) error {
	hasOverlays, err := s.repo.HasOverlays(ctx, s.pool, schemaID)
	if err != nil {
		return fmt.Errorf("checking overlays for schema %s: %w", schemaID, err)
	}
	if hasOverlays {
		return apierr.BadRequest("system overlay schema has associated property overlays and cannot be deleted")
	}

	err = db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, schemaID)
		if err != nil {
			return err
		}
		if existing.SystemID != systemID {
			return ErrNotFound
		}
		beforeJSON, _ := json.Marshal(existing)

		if err := s.repo.Delete(ctx, tx, schemaID); err != nil {
			return err
		}
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpDelete,
			ResourceType: "system_overlay_schema",
			ResourceID:   schemaID,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
}

// resolveSystemName looks up the system name required for namespace validation.
// Returns an *apierr.APIError when the system does not exist.
func (s *Service) resolveSystemName(ctx context.Context, systemID uuid.UUID) (string, *apierr.APIError) {
	var name string
	err := s.pool.QueryRow(ctx,
		`SELECT name FROM system WHERE id = $1`, systemID,
	).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apierr.NotFound("system")
		}
		return "", apierr.Internal("failed to resolve system")
	}
	return name, nil
}

// validateSchema validates that allowed_overlay_properties is a valid JSON Schema
// and that all declared property keys carry the required namespace prefix (FR-OVS-004, FR-OVS-005).
func validateSchema(schema json.RawMessage, systemName string) *apierr.APIError {
	if apiErr := validation.ValidateIsJSONSchema(schema); apiErr != nil {
		return apierr.ValidationFailed(
			append([]string{"allowed_overlay_properties: invalid JSON Schema"}, apiErr.Details...)...,
		)
	}
	return validateNamespacePrefix(schema, systemName)
}

// validateNamespacePrefix checks that all property keys declared in the JSON Schema
// are prefixed with "{systemName}." to prevent key collisions (FR-OVS-005).
func validateNamespacePrefix(schema json.RawMessage, systemName string) *apierr.APIError {
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return apierr.BadRequest("invalid JSON in allowed_overlay_properties: " + err.Error())
	}

	if len(doc.Properties) == 0 {
		return nil
	}

	prefix := systemName + "."
	var violations []string
	for key := range doc.Properties {
		if !strings.HasPrefix(key, prefix) {
			violations = append(violations, "'"+key+"'")
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return apierr.ValidationFailed(fmt.Sprintf(
			"property keys must be prefixed with %q: %s",
			prefix, strings.Join(violations, ", "),
		))
	}
	return nil
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
		return apierr.NotFound("system overlay schema")
	case errors.Is(err, ErrDuplicate):
		return apierr.Conflict("system overlay schema already exists for this system and entity type")
	case errors.Is(err, ErrSystemNotFound):
		return apierr.NotFound("system")
	case errors.Is(err, ErrEntityTypeNotFound):
		return apierr.NotFound("entity type definition")
	case errors.Is(err, ErrHasOverlays):
		return apierr.BadRequest("system overlay schema has associated property overlays and cannot be deleted")
	}
	return err
}
