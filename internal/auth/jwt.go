package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type Provider struct {
	JWKSURL       string
	Issuer        string
	Audience      string
	ClaimsMapping ClaimsMapping
}

// ClaimsMapping adapts an IdP's native token claims to OAD's identity model.
type ClaimsMapping struct {
	RolesClaim    string   // defaults to "oad_roles"
	SystemIDClaim string   // defaults to "oad_system_id"
	DefaultRoles  []string // used when RolesClaim is absent from the token
}

// JWTAuthenticator validates JWT Bearer tokens using per-provider JWKS endpoints.
// It maintains an auto-refreshing key cache so that key rotations at the
// identity provider are picked up without restarts.
type JWTAuthenticator struct {
	cache     *jwk.Cache
	providers map[string]Provider // keyed by Issuer
}

// NewJWTAuthenticator creates a JWT authenticator for one or more trusted
// identity providers. Each provider carries its own JWKS URL, expected issuer,
// and expected audience, enabling independent key rotation and audience isolation.
// It performs an initial key fetch per provider to fail fast at startup if any
// OIDC provider is unreachable.
func NewJWTAuthenticator(ctx context.Context, providers []Provider) (*JWTAuthenticator, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	cache := jwk.NewCache(ctx)
	byIssuer := make(map[string]Provider, len(providers))

	for _, p := range providers {
		if err := cache.Register(p.JWKSURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
			return nil, fmt.Errorf("registering JWKS URL %q: %w", p.JWKSURL, err)
		}
		if _, err := cache.Refresh(ctx, p.JWKSURL); err != nil {
			return nil, fmt.Errorf("initial JWKS fetch from %s: %w", p.JWKSURL, err)
		}
		byIssuer[p.Issuer] = p
	}

	return &JWTAuthenticator{
		cache:     cache,
		providers: byIssuer,
	}, nil
}

// Authenticate parses and validates a raw JWT string. It reads the issuer claim
// first (without signature verification) to select the correct provider, then
// verifies the signature and all standard claims (exp, iss, aud) against that
// provider's JWKS and expected audience. Finally it extracts OAD-specific
// custom claims (oad_roles, oad_system_id).
func (a *JWTAuthenticator) Authenticate(ctx context.Context, tokenString string) (*Identity, error) {
	// Parse without validation only to read the issuer claim.
	raw, err := jwt.ParseInsecure([]byte(tokenString))
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}

	p, ok := a.providers[raw.Issuer()]
	if !ok {
		return nil, fmt.Errorf("untrusted token issuer %q", raw.Issuer())
	}

	keySet, err := a.cache.Get(ctx, p.JWKSURL)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS for issuer %q: %w", p.Issuer, err)
	}

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(p.Issuer),
		jwt.WithAudience(p.Audience),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	sub := token.Subject()
	if sub == "" {
		return nil, errors.New("token missing required 'sub' claim")
	}

	rolesClaim := p.ClaimsMapping.RolesClaim
	if rolesClaim == "" {
		rolesClaim = "oad_roles"
	}
	roles, err := extractStringSlice(token, rolesClaim)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 && len(p.ClaimsMapping.DefaultRoles) > 0 {
		roles = p.ClaimsMapping.DefaultRoles
	}

	systemIDClaim := p.ClaimsMapping.SystemIDClaim
	if systemIDClaim == "" {
		systemIDClaim = "oad_system_id"
	}
	systemID, _ := extractString(token, systemIDClaim)

	return &Identity{
		Subject:  sub,
		Roles:    roles,
		SystemID: systemID,
		AuthMode: "jwt",
	}, nil
}

// extractStringSlice reads a custom claim as a []string.
func extractStringSlice(token jwt.Token, claim string) ([]string, error) {
	raw, ok := token.Get(claim)
	if !ok {
		return nil, nil
	}

	rawSlice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("claim %q: expected array, got %T", claim, raw)
	}

	result := make([]string, 0, len(rawSlice))
	for _, v := range rawSlice {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("claim %q: expected string element, got %T", claim, v)
		}
		result = append(result, s)
	}
	return result, nil
}

// extractString reads a custom claim as a string.
func extractString(token jwt.Token, claim string) (string, bool) {
	raw, ok := token.Get(claim)
	if !ok {
		return "", false
	}
	s, ok := raw.(string)
	return s, ok
}
