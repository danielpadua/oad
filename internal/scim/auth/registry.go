// Package auth implements SCIM tenant token authentication.
//
// Each configured provider may carry its own bearer token. Tokens are loaded
// once at startup, hashed with SHA-256, and stored in a Registry. The
// registry resolves an incoming bearer token to its provider name; the
// plaintext is not retained beyond Register().
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

// ErrEmptyProvider is returned by Register when an empty provider name is supplied.
var ErrEmptyProvider = errors.New("scim auth: provider name required")

// ErrEmptyToken is returned by Register when an empty token is supplied.
var ErrEmptyToken = errors.New("scim auth: token required")

// ErrDuplicateToken is returned by Register when the same token is already registered.
// This indicates a misconfiguration: two providers must not share a token.
var ErrDuplicateToken = errors.New("scim auth: token already registered for another provider")

// Registry maps SCIM tenant tokens to their provider name. Tokens are
// stored as SHA-256 hashes; plaintext is dropped after Register returns.
//
// Registry is safe for concurrent reads after all Register calls complete
// during startup. Mutating the registry after the HTTP server has started
// is not supported.
type Registry struct {
	entries []entry
}

type entry struct {
	provider string
	hash     [sha256.Size]byte
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register associates a plaintext bearer token with a provider name.
// Returns ErrEmptyProvider, ErrEmptyToken, or ErrDuplicateToken on
// validation failures.
func (r *Registry) Register(provider, token string) error {
	if provider == "" {
		return ErrEmptyProvider
	}
	if token == "" {
		return ErrEmptyToken
	}
	hash := sha256.Sum256([]byte(token))
	for _, e := range r.entries {
		if subtle.ConstantTimeCompare(hash[:], e.hash[:]) == 1 {
			return ErrDuplicateToken
		}
	}
	r.entries = append(r.entries, entry{provider: provider, hash: hash})
	return nil
}

// Lookup returns the provider name associated with the given bearer token,
// or ("", false) if the token is empty or not registered. Comparison uses
// crypto/subtle.ConstantTimeCompare on the SHA-256 hashes; total runtime
// scales linearly with the number of registered providers.
func (r *Registry) Lookup(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	candidate := sha256.Sum256([]byte(token))
	for _, e := range r.entries {
		if subtle.ConstantTimeCompare(candidate[:], e.hash[:]) == 1 {
			return e.provider, true
		}
	}
	return "", false
}

// Size returns the number of registered providers. Useful for startup logs
// and for tests.
func (r *Registry) Size() int {
	return len(r.entries)
}
