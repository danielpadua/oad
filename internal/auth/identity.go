// Package auth provides authentication and identity management for the OAD API.
// It owns the Identity type and context key, imported by middleware, handlers,
// and the db/audit packages without creating import cycles.
package auth

import "context"

// Identity represents the authenticated caller's claims extracted from a JWT
// or mTLS client certificate.
type Identity struct {
	Subject  string   // JWT "sub" claim or mTLS certificate CN.
	Roles    []string // Application roles from "oad_roles" claim (admin, editor, viewer).
	SystemID string   // From "oad_system_id" claim; empty string = platform admin.
	AuthMode string   // "jwt" or "mtls" — for audit and diagnostics.
}

// HasRole reports whether the identity holds the given role.
func (id *Identity) HasRole(role string) bool {
	for _, r := range id.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the identity holds at least one of the listed roles.
func (id *Identity) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if id.HasRole(role) {
			return true
		}
	}
	return false
}

type contextKey string

const identityKey contextKey = "identity"

// WithIdentity returns a new context carrying the given identity.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext retrieves the authenticated identity from the context.
// Returns nil and false if no identity is present.
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityKey).(*Identity)
	return id, ok
}

// MustIdentityFromContext retrieves the identity from the context or panics.
// Use only in code paths where the authentication middleware guarantees presence.
func MustIdentityFromContext(ctx context.Context) *Identity {
	id, ok := IdentityFromContext(ctx)
	if !ok {
		panic("auth: identity not found in context — authentication middleware missing")
	}
	return id
}
