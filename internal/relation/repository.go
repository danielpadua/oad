package relation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/danielpadua/oad/internal/db"
)

// Repository abstracts the persistence layer for relations.
// Write methods accept pgx.Tx to guarantee atomicity with audit log writes.
// Read methods accept db.DBTX to work both inside and outside transactions.
type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, rel *Relation) error
	GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Relation, error)
	ListByEntity(ctx context.Context, q db.DBTX, entityID uuid.UUID, params ListParams) ([]*Relation, int64, error)
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

type pgxRepository struct{}

// NewRepository returns the default PostgreSQL-backed repository.
func NewRepository() Repository {
	return &pgxRepository{}
}

func scanRelation(row pgx.Row) (*Relation, error) {
	rel := &Relation{}
	err := row.Scan(
		&rel.ID, &rel.SubjectEntityID, &rel.RelationType,
		&rel.TargetEntityID, &rel.SystemID, &rel.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning relation: %w", err)
	}
	return rel, nil
}

func (r *pgxRepository) Create(ctx context.Context, tx pgx.Tx, rel *Relation) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO relation (subject_entity_id, relation_type, target_entity_id, system_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		rel.SubjectEntityID, rel.RelationType, rel.TargetEntityID, rel.SystemID,
	).Scan(&rel.ID, &rel.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("inserting relation: %w", err)
	}
	return nil
}

func (r *pgxRepository) GetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*Relation, error) {
	row := q.QueryRow(ctx,
		`SELECT id, subject_entity_id, relation_type, target_entity_id, system_id, created_at
		 FROM relation
		 WHERE id = $1`,
		id,
	)
	return scanRelation(row)
}

func (r *pgxRepository) ListByEntity(ctx context.Context, q db.DBTX, entityID uuid.UUID, params ListParams) ([]*Relation, int64, error) {
	args := []any{entityID}
	conds := []string{"r.subject_entity_id = $1"}

	if params.RelationType != "" {
		args = append(args, params.RelationType)
		conds = append(conds, fmt.Sprintf("r.relation_type = $%d", len(args)))
	}
	if params.SystemID != nil {
		args = append(args, params.SystemID)
		conds = append(conds, fmt.Sprintf("r.system_id = $%d", len(args)))
	}

	whereClause := strings.Join(conds, " AND ")
	baseFrom := `FROM relation r WHERE ` + whereClause

	var total int64
	if err := q.QueryRow(ctx, "SELECT COUNT(*) "+baseFrom, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting relations: %w", err)
	}

	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, params.Limit, params.Offset)

	listSQL := fmt.Sprintf(
		`SELECT r.id, r.subject_entity_id, r.relation_type, r.target_entity_id, r.system_id, r.created_at
		 %s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`,
		baseFrom, len(listArgs)-1, len(listArgs),
	)

	rows, err := q.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying relations: %w", err)
	}
	defer rows.Close()

	result := []*Relation{}
	for rows.Next() {
		rel := &Relation{}
		if err := rows.Scan(&rel.ID, &rel.SubjectEntityID, &rel.RelationType,
			&rel.TargetEntityID, &rel.SystemID, &rel.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning relation: %w", err)
		}
		result = append(result, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating relations: %w", err)
	}
	return result, total, nil
}

func (r *pgxRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, `DELETE FROM relation WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting relation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
