package retrieval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts all retrieval-specific DB operations.
// Read methods accept db.DBTX to work with both a pool and an active transaction.
type Repository interface {
	// LookupMerged retrieves an entity by type + external_id, merging in the
	// system's property overlay and the appropriate set of relations when
	// a system context is provided.
	LookupMerged(ctx context.Context, q db.DBTX, params LookupParams) (*MergedEntityView, error)

	// FilterByProperties returns entities whose properties satisfy a JSONB
	// containment filter, leveraging the GIN index on entity.properties.
	FilterByProperties(ctx context.Context, q db.DBTX, params FilterParams) ([]*MergedEntityView, int64, error)

	// ListChangelog returns a paginated, time-ordered slice of audit_log rows
	// since the given timestamp, optionally scoped to a system.
	ListChangelog(ctx context.Context, q db.DBTX, params ChangelogParams) ([]*ChangelogEntry, int64, error)

	// ExportEntities returns a deterministically-ordered page of entities with
	// their relations attached (batch-loaded to avoid N+1 queries).
	ExportEntities(ctx context.Context, q db.DBTX, params ExportParams) ([]*ExportItem, int64, error)

	// WriteLog inserts a retrieval_log row.
	WriteLog(ctx context.Context, q db.DBTX, entry LogEntry) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed retrieval repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

// LookupMerged performs three sequential reads:
//  1. Fetch the entity base row.
//  2. If a system context is given, fetch the property overlay and merge it.
//  3. Fetch relations — global always; system-scoped when a system context is given.
func (r *pgxRepository) LookupMerged(ctx context.Context, q db.DBTX, params LookupParams) (*MergedEntityView, error) {
	view := &MergedEntityView{}
	err := q.QueryRow(ctx,
		`SELECT e.id, e.type_id, etd.type_name, e.external_id,
		        e.properties, e.system_id, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE etd.type_name = $1 AND e.external_id = $2`,
		params.TypeName, params.ExternalID,
	).Scan(
		&view.ID, &view.TypeID, &view.Type, &view.ExternalID,
		&view.Properties, &view.SystemID, &view.CreatedAt, &view.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEntityNotFound
		}
		return nil, fmt.Errorf("looking up entity: %w", err)
	}

	// Merge property overlay when a system context is requested (FR-OVL-006).
	if params.SystemID != nil {
		var overlayProps json.RawMessage
		overlayErr := q.QueryRow(ctx,
			`SELECT properties FROM property_overlay
			 WHERE entity_id = $1 AND system_id = $2`,
			view.ID, params.SystemID,
		).Scan(&overlayProps)
		// pgx.ErrNoRows means no overlay exists; use global properties as-is.
		if overlayErr == nil && len(overlayProps) > 0 {
			view.Properties = mergeJSON(view.Properties, overlayProps)
		}
	}

	// Fetch relations: global relations are always included; system-scoped
	// relations are included only when a system context is provided (FR-OVL-007).
	relRows, err := q.Query(ctx,
		`SELECT id, relation_type, target_entity_id, system_id
		 FROM relation
		 WHERE subject_entity_id = $1
		   AND (system_id IS NULL OR ($2::uuid IS NOT NULL AND system_id = $2))
		 ORDER BY relation_type, created_at`,
		view.ID, params.SystemID,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching relations for lookup: %w", err)
	}
	defer relRows.Close()

	view.Relations = []RelationRef{}
	for relRows.Next() {
		var rel RelationRef
		if err := relRows.Scan(&rel.ID, &rel.RelationType, &rel.TargetEntityID, &rel.SystemID); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		view.Relations = append(view.Relations, rel)
	}
	if err := relRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating relations: %w", err)
	}

	return view, nil
}

// FilterByProperties queries entities by JSONB containment (GIN index — NFR-PRF-001).
// The filter must be a valid JSON object; e.g. {"department":"ops"}.
func (r *pgxRepository) FilterByProperties(ctx context.Context, q db.DBTX, params FilterParams) ([]*MergedEntityView, int64, error) {
	var typeName *string
	if params.TypeName != "" {
		typeName = &params.TypeName
	}

	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.properties @> $1::jsonb
		   AND ($2::text IS NULL OR etd.type_name = $2)`,
		params.Filter, typeName,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting filtered entities: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT e.id, e.type_id, etd.type_name, e.external_id,
		        e.properties, e.system_id, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE e.properties @> $1::jsonb
		   AND ($2::text IS NULL OR etd.type_name = $2)
		 ORDER BY e.created_at DESC, e.id ASC
		 LIMIT $3 OFFSET $4`,
		params.Filter, typeName, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("filtering entities by properties: %w", err)
	}
	defer rows.Close()

	result := []*MergedEntityView{}
	for rows.Next() {
		v := &MergedEntityView{Relations: []RelationRef{}}
		if err := rows.Scan(&v.ID, &v.TypeID, &v.Type, &v.ExternalID,
			&v.Properties, &v.SystemID, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning filtered entity: %w", err)
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating filtered entities: %w", err)
	}
	return result, total, nil
}

// ListChangelog returns audit_log rows since the given timestamp. When a system_id
// is provided, the result includes entries scoped to that system plus entries with
// no system scope (global operations). Admins (nil system_id) see all entries.
func (r *pgxRepository) ListChangelog(ctx context.Context, q db.DBTX, params ChangelogParams) ([]*ChangelogEntry, int64, error) {
	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM audit_log
		 WHERE timestamp > $1
		   AND ($2::uuid IS NULL OR system_id IS NULL OR system_id = $2)`,
		params.Since, params.SystemID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting changelog entries: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT id, actor, operation, resource_type, resource_id,
		        before_value, after_value, system_id, timestamp
		 FROM audit_log
		 WHERE timestamp > $1
		   AND ($2::uuid IS NULL OR system_id IS NULL OR system_id = $2)
		 ORDER BY timestamp ASC
		 LIMIT $3 OFFSET $4`,
		params.Since, params.SystemID, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying changelog: %w", err)
	}
	defer rows.Close()

	result := []*ChangelogEntry{}
	for rows.Next() {
		e := &ChangelogEntry{}
		if err := rows.Scan(
			&e.ID, &e.Actor, &e.Operation, &e.ResourceType, &e.ResourceID,
			&e.BeforeValue, &e.AfterValue, &e.SystemID, &e.Timestamp,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning changelog entry: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating changelog: %w", err)
	}
	return result, total, nil
}

// ExportEntities returns a deterministically-ordered page of entities (FR-RET-004).
// Relations are batch-loaded in a second query to avoid N+1 per entity.
// When params.SystemID is nil (admin), all relations are included; otherwise only
// global and system-scoped relations for the specified system are included.
func (r *pgxRepository) ExportEntities(ctx context.Context, q db.DBTX, params ExportParams) ([]*ExportItem, int64, error) {
	var typeName *string
	if params.TypeName != "" {
		typeName = &params.TypeName
	}

	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE ($1::text IS NULL OR etd.type_name = $1)`,
		typeName,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting export entities: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT e.id, e.type_id, etd.type_name, e.external_id,
		        e.properties, e.system_id, e.created_at, e.updated_at
		 FROM entity e
		 JOIN entity_type_definition etd ON etd.id = e.type_id
		 WHERE ($1::text IS NULL OR etd.type_name = $1)
		 ORDER BY e.created_at ASC, e.id ASC
		 LIMIT $2 OFFSET $3`,
		typeName, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("exporting entities: %w", err)
	}
	defer rows.Close()

	items := []*ExportItem{}
	entityIDs := []uuid.UUID{}
	itemByID := map[uuid.UUID]*ExportItem{}

	for rows.Next() {
		item := &ExportItem{Relations: []RelationRef{}}
		if err := rows.Scan(
			&item.ID, &item.TypeID, &item.Type, &item.ExternalID,
			&item.Properties, &item.SystemID, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning export entity: %w", err)
		}
		items = append(items, item)
		entityIDs = append(entityIDs, item.ID)
		itemByID[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating export entities: %w", err)
	}

	if len(entityIDs) == 0 {
		return items, total, nil
	}

	// Batch-load relations for the entity page to avoid N+1 queries.
	// Admin callers (nil system_id) get all relations; scoped callers get
	// global and their own system-scoped relations.
	relRows, err := q.Query(ctx,
		`SELECT r.id, r.subject_entity_id, r.relation_type, r.target_entity_id, r.system_id
		 FROM relation r
		 WHERE r.subject_entity_id = ANY($1)
		   AND ($2::uuid IS NULL OR r.system_id IS NULL OR r.system_id = $2)
		 ORDER BY r.subject_entity_id, r.relation_type, r.created_at`,
		entityIDs, params.SystemID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("fetching relations for export: %w", err)
	}
	defer relRows.Close()

	for relRows.Next() {
		var subjectID uuid.UUID
		var rel RelationRef
		if err := relRows.Scan(&rel.ID, &subjectID, &rel.RelationType, &rel.TargetEntityID, &rel.SystemID); err != nil {
			return nil, 0, fmt.Errorf("scanning export relation: %w", err)
		}
		if item, ok := itemByID[subjectID]; ok {
			item.Relations = append(item.Relations, rel)
		}
	}
	if err := relRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating export relations: %w", err)
	}

	return items, total, nil
}

// WriteLog inserts a retrieval_log row (FR-AUD-002).
func (r *pgxRepository) WriteLog(ctx context.Context, q db.DBTX, entry LogEntry) error {
	_, err := q.Exec(ctx,
		`INSERT INTO retrieval_log (caller_identity, query_parameters, returned_refs, system_id)
		 VALUES ($1, $2, $3, $4)`,
		entry.CallerIdentity,
		entry.QueryParams,
		entry.ReturnedRefs,
		entry.SystemID,
	)
	if err != nil {
		return fmt.Errorf("writing retrieval log: %w", err)
	}
	return nil
}

// mergeJSON merges overlay on top of base using the same semantics as PostgreSQL's
// || JSONB operator (overlay keys win on conflict). Returns base unchanged on any error.
func mergeJSON(base, overlay json.RawMessage) json.RawMessage {
	if len(overlay) == 0 {
		return base
	}
	var baseMap map[string]json.RawMessage
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return base
	}
	if baseMap == nil {
		baseMap = make(map[string]json.RawMessage)
	}
	var overlayMap map[string]json.RawMessage
	if err := json.Unmarshal(overlay, &overlayMap); err != nil {
		return base
	}
	maps.Copy(baseMap, overlayMap)
	merged, err := json.Marshal(baseMap)
	if err != nil {
		return base
	}
	return merged
}
