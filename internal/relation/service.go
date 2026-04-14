package relation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/db"
)

// allowedRelationEntry mirrors the structure within entity_type_definition.allowed_relations:
// {"member": {"target_types": ["group", "role"]}}.
type allowedRelationEntry struct {
	TargetTypes []string `json:"target_types"`
}

// subjectInfo holds the data about the subject entity needed for relation validation.
type subjectInfo struct {
	TypeName         string
	AllowedRelations json.RawMessage
}

// Service orchestrates business logic for relation management.
// Every mutation runs inside a transaction that also writes the audit record,
// guaranteeing that no mutation occurs without a corresponding audit entry (NFR-AUD-001).
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new relation service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create validates and persists a new relation (FR-REL-001, FR-REL-002, FR-REL-003).
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Relation, error) {
	if apiErr := validateCreateRequest(req); apiErr != nil {
		return nil, apiErr
	}

	var result *Relation
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		// Resolve subject entity and its type definition (FR-REL-002).
		subject, apiErr := s.getSubjectInfo(ctx, tx, req.SubjectEntityID)
		if apiErr != nil {
			return apiErr
		}

		// Validate relation_type against the subject's allowed_relations.
		entry, apiErr := extractRelationEntry(subject, req.RelationType)
		if apiErr != nil {
			return apiErr
		}

		// Resolve target entity type name.
		targetTypeName, apiErr := s.getTargetTypeName(ctx, tx, req.TargetEntityID)
		if apiErr != nil {
			return apiErr
		}

		// Validate target entity type is allowed for this relation_type.
		if !slices.Contains(entry.TargetTypes, targetTypeName) {
			return apierr.BadRequest(fmt.Sprintf(
				"entity type '%s' is not a valid target for relation_type '%s' on type '%s'",
				targetTypeName, req.RelationType, subject.TypeName,
			))
		}

		rel := &Relation{
			SubjectEntityID: req.SubjectEntityID,
			RelationType:    req.RelationType,
			TargetEntityID:  req.TargetEntityID,
			SystemID:        req.SystemID,
		}
		if err := s.repo.Create(ctx, tx, rel); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(rel)
		result = rel
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "relation",
			ResourceID:   rel.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return result, nil
}

// GetByID retrieves a single relation by its UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Relation, error) {
	rel, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return rel, nil
}

// ListByEntity returns a paginated list of relations where the given entity is the
// subject, optionally filtered by relation_type and system_id (FR-REL-005).
func (s *Service) ListByEntity(ctx context.Context, entityID uuid.UUID, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.ListByEntity(ctx, s.pool, entityID, params)
	if err != nil {
		return nil, fmt.Errorf("listing relations: %w", err)
	}
	return &ListResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// Delete removes a relation by ID (FR-REL-004).
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
			ResourceType: "relation",
			ResourceID:   id,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
}

// getSubjectInfo retrieves the subject entity's type name and allowed_relations
// from the entity and entity_type_definition tables within the given transaction.
func (s *Service) getSubjectInfo(ctx context.Context, q db.DBTX, entityID uuid.UUID) (*subjectInfo, *apierr.APIError) {
	info := &subjectInfo{}
	err := q.QueryRow(ctx,
		`SELECT etd.type_name, etd.allowed_relations
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.id = $1`,
		entityID,
	).Scan(&info.TypeName, &info.AllowedRelations)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierr.NotFound("subject entity")
		}
		return nil, apierr.Internal("failed to resolve subject entity")
	}
	return info, nil
}

// getTargetTypeName resolves the type name of the target entity.
func (s *Service) getTargetTypeName(ctx context.Context, q db.DBTX, entityID uuid.UUID) (string, *apierr.APIError) {
	var typeName string
	err := q.QueryRow(ctx,
		`SELECT etd.type_name
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.id = $1`,
		entityID,
	).Scan(&typeName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apierr.NotFound("target entity")
		}
		return "", apierr.Internal("failed to resolve target entity")
	}
	return typeName, nil
}

// extractRelationEntry looks up a relation_type in the subject's allowed_relations map.
func extractRelationEntry(subject *subjectInfo, relationType string) (*allowedRelationEntry, *apierr.APIError) {
	var allowedRelations map[string]allowedRelationEntry
	if err := json.Unmarshal(subject.AllowedRelations, &allowedRelations); err != nil {
		return nil, apierr.Internal("failed to parse allowed_relations")
	}
	entry, ok := allowedRelations[relationType]
	if !ok {
		return nil, apierr.BadRequest(fmt.Sprintf(
			"relation_type '%s' is not declared for entity type '%s'",
			relationType, subject.TypeName,
		))
	}
	return &entry, nil
}

// validateCreateRequest checks required fields before expensive DB lookups.
func validateCreateRequest(req CreateRequest) *apierr.APIError {
	if req.SubjectEntityID == uuid.Nil {
		return apierr.BadRequest("subject_entity_id is required")
	}
	if req.RelationType == "" {
		return apierr.BadRequest("relation_type is required")
	}
	if req.TargetEntityID == uuid.Nil {
		return apierr.BadRequest("target_entity_id is required")
	}
	if req.SubjectEntityID == req.TargetEntityID {
		return apierr.BadRequest("subject_entity_id and target_entity_id must be different")
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
		return apierr.NotFound("relation")
	case errors.Is(err, ErrDuplicate):
		return apierr.Conflict("relation already exists")
	}
	return err
}
