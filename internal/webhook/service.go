package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/audit"
	"github.com/danielpadua/oad/internal/db"
)

// Service orchestrates business logic for webhook subscription management.
// All write operations run inside a transaction that also writes the audit record.
type Service struct {
	pool  *pgxpool.Pool
	repo  Repository
	audit *audit.Service
}

// NewService creates a new webhook subscription service.
func NewService(pool *pgxpool.Pool, repo Repository, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, repo: repo, audit: auditSvc}
}

// Create registers a new webhook subscription for the given system (FR-WHK-001).
// The callback URL is validated; the HMAC secret is stored and never returned again.
func (s *Service) Create(ctx context.Context, systemID uuid.UUID, req CreateRequest) (*Subscription, error) {
	if req.CallbackURL == "" {
		return nil, apierr.BadRequest("callback_url is required")
	}
	if _, err := url.ParseRequestURI(req.CallbackURL); err != nil {
		return nil, apierr.BadRequest("callback_url must be a valid URL: " + err.Error())
	}
	if req.Secret == "" {
		return nil, apierr.BadRequest("secret is required")
	}
	if len(req.Secret) < 16 {
		return nil, apierr.BadRequest("secret must be at least 16 characters")
	}

	sub := &Subscription{
		SystemID:    systemID,
		CallbackURL: req.CallbackURL,
		Active:      true,
	}

	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, sub, req.Secret); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(sub)
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpCreate,
			ResourceType: "webhook_subscription",
			ResourceID:   sub.ID,
			AfterValue:   afterJSON,
		})
	})
	if err != nil {
		return nil, toAPIError(err)
	}
	return sub, nil
}

// GetByID retrieves a single webhook subscription by ID (FR-WHK-003).
// The subscription must belong to the specified system.
func (s *Service) GetByID(ctx context.Context, systemID, subscriptionID uuid.UUID) (*Subscription, error) {
	sub, err := s.repo.GetByID(ctx, s.pool, subscriptionID)
	if err != nil {
		return nil, toAPIError(err)
	}
	if sub.SystemID != systemID {
		return nil, apierr.NotFound("webhook subscription")
	}
	return sub, nil
}

// List returns a paginated list of webhook subscriptions for the given system (FR-WHK-003).
func (s *Service) List(ctx context.Context, systemID uuid.UUID, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.List(ctx, s.pool, systemID, params)
	if err != nil {
		return nil, fmt.Errorf("listing webhook subscriptions: %w", err)
	}
	if items == nil {
		items = []*Subscription{}
	}
	return &ListResult{
		Items:  items,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// Update applies a partial update to an existing subscription (FR-WHK-003).
// The subscription must belong to the specified system.
func (s *Service) Update(ctx context.Context, systemID, subscriptionID uuid.UUID, req UpdateRequest) (*Subscription, error) {
	if req.CallbackURL != nil && *req.CallbackURL == "" {
		return nil, apierr.BadRequest("callback_url must not be empty")
	}
	if req.CallbackURL != nil {
		if _, err := url.ParseRequestURI(*req.CallbackURL); err != nil {
			return nil, apierr.BadRequest("callback_url must be a valid URL: " + err.Error())
		}
	}

	var result *Subscription
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		if existing.SystemID != systemID {
			return ErrNotFound
		}
		beforeJSON, _ := json.Marshal(existing)

		if req.CallbackURL != nil {
			existing.CallbackURL = *req.CallbackURL
		}
		if req.Active != nil {
			existing.Active = *req.Active
		}

		if err := s.repo.Update(ctx, tx, existing); err != nil {
			return err
		}
		afterJSON, _ := json.Marshal(existing)
		result = existing
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpUpdate,
			ResourceType: "webhook_subscription",
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

// Delete removes a webhook subscription and all its delivery records (FR-WHK-003).
// The subscription must belong to the specified system.
func (s *Service) Delete(ctx context.Context, systemID, subscriptionID uuid.UUID) error {
	err := db.WithAuthScope(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetByID(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		if existing.SystemID != systemID {
			return ErrNotFound
		}
		beforeJSON, _ := json.Marshal(existing)

		if err := s.repo.Delete(ctx, tx, subscriptionID); err != nil {
			return err
		}
		return s.audit.WriteFromContext(ctx, tx, audit.Entry{
			Operation:    audit.OpDelete,
			ResourceType: "webhook_subscription",
			ResourceID:   subscriptionID,
			BeforeValue:  beforeJSON,
		})
	})
	return toAPIError(err)
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
	if errors.Is(err, ErrNotFound) {
		return apierr.NotFound("webhook subscription")
	}
	return err
}
