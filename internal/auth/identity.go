// Package auth provides authentication and identity management for the OAD API.
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Identity represents the fully-resolved caller identity for a single request.
// For JWT callers, it is populated by the IdentityResolver from the entity/relation graph.
// For mTLS callers, it is populated directly from the client certificate.
type Identity struct {
	Subject         string      // JWT "sub" or mTLS certificate CN — preserved for audit traceability.
	Provider        string      // Matched auth.providers[].name; "mtls" for mTLS callers.
	EntityID        uuid.UUID   // entity.id of the User; zero for mTLS callers without a DB entity.
	Groups          []string    // external_id values of all Groups the user is member_of.
	AllowedSystems  []uuid.UUID // entity.id of every System the user can access.
	IsPlatformAdmin bool        // true iff "oad:admin" is in Groups (or mTLS cert has admin OU).
	ActiveSystemID  *uuid.UUID  // Selected via X-OAD-System-Id header; nil on non-scoped routes.
	AuthMode        string      // "jwt" or "mtls" — for audit and diagnostics.
}

// HasRole reports whether the identity satisfies the named role.
// Roles map to built-in group memberships:
//
//	"admin"  → IsPlatformAdmin
//	"editor" → IsPlatformAdmin or "oad:editor" in Groups
//	"viewer" → IsPlatformAdmin or "oad:editor" or "oad:viewer" in Groups
func (id *Identity) HasRole(role string) bool {
	switch role {
	case "admin":
		return id.IsPlatformAdmin
	case "editor":
		return id.IsPlatformAdmin || id.hasGroup("oad:editor")
	case "viewer":
		return id.IsPlatformAdmin || id.hasGroup("oad:editor") || id.hasGroup("oad:viewer")
	}
	return false
}

// HasAnyRole reports whether the identity satisfies at least one of the listed roles.
func (id *Identity) HasAnyRole(roles ...string) bool {
	for _, r := range roles {
		if id.HasRole(r) {
			return true
		}
	}
	return false
}

func (id *Identity) hasGroup(externalID string) bool {
	for _, g := range id.Groups {
		if g == externalID {
			return true
		}
	}
	return false
}

// ActorString returns the canonical actor string for audit log entries.
// DB-provisioned users are identified by entity ID; mTLS callers by subject.
func (id *Identity) ActorString() string {
	if id.EntityID != (uuid.UUID{}) {
		return "user:" + id.EntityID.String()
	}
	return id.Subject
}

type contextKey string

const identityKey contextKey = "identity"

// WithIdentity returns a new context carrying the given identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext retrieves the authenticated identity from the context.
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityKey).(*Identity)
	return id, ok
}

// MustIdentityFromContext retrieves the identity or panics if absent.
func MustIdentityFromContext(ctx context.Context) *Identity {
	id, ok := IdentityFromContext(ctx)
	if !ok {
		panic("auth: identity not found in context — authentication middleware missing")
	}
	return id
}
