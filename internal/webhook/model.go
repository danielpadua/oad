// Package webhook manages webhook subscriptions and asynchronous event delivery.
// A subscription binds a system to a consumer callback URL; the dispatcher
// delivers HMAC-signed event payloads for every attribute change within that system.
// Implements FR-WHK-001 through FR-WHK-004.
package webhook

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subscription is the API-facing model for a webhook subscription.
// The HMAC-SHA256 signing secret is stored in the database but never returned
// in any API response to prevent secret exposure in logs or responses (NFR-SEC-007).
type Subscription struct {
	ID          uuid.UUID `json:"id"`
	SystemID    uuid.UUID `json:"system_id"`
	CallbackURL string    `json:"callback_url"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateRequest is the payload for registering a new webhook subscription (FR-WHK-001).
// The caller-supplied secret is stored as the HMAC-SHA256 signing key and is
// never echoed back in any subsequent response.
type CreateRequest struct {
	CallbackURL string `json:"callback_url"`
	Secret      string `json:"secret"`
}

// UpdateRequest applies a partial update to an existing subscription (FR-WHK-003).
// Omitted fields are left unchanged.
type UpdateRequest struct {
	CallbackURL *string `json:"callback_url,omitempty"`
	Active      *bool   `json:"active,omitempty"`
}

// ListParams controls pagination for subscription list queries.
type ListParams struct {
	Limit  int
	Offset int
}

// ListResult is a paginated list of subscriptions.
type ListResult struct {
	Items  []*Subscription `json:"items"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// PendingDelivery joins a webhook_delivery row with the associated
// webhook_subscription and audit_log data required to build, sign, and dispatch
// the outgoing event payload (FR-WHK-002, FR-WHK-004).
type PendingDelivery struct {
	DeliveryID     uuid.UUID
	SubscriptionID uuid.UUID
	AuditLogID     uuid.UUID
	Attempts       int
	CallbackURL    string
	Secret         string // HMAC-SHA256 signing key; never logged
	Actor          string
	Operation      string
	ResourceType   string
	ResourceID     uuid.UUID
	BeforeValue    json.RawMessage
	AfterValue     json.RawMessage
	SystemID       *uuid.UUID
	AuditTimestamp time.Time
}

// EventPayload is the JSON body posted to subscriber callback URLs (FR-WHK-002).
// EventType follows the pattern "{resource_type}.{operation}" (e.g. "entity.created").
type EventPayload struct {
	EventID      uuid.UUID       `json:"event_id"`
	EventType    string          `json:"event_type"`
	SystemID     *uuid.UUID      `json:"system_id,omitempty"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uuid.UUID       `json:"resource_id"`
	Actor        string          `json:"actor"`
	Timestamp    time.Time       `json:"timestamp"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
}
