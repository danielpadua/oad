package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// JWTAuthenticator validates JWT Bearer tokens using a remote JWKS endpoint.
// It maintains an auto-refreshing key cache so that key rotations at the
// identity provider are picked up without restarts.
type JWTAuthenticator struct {
	cache    *jwk.Cache
	jwksURL  string
	audience string
	issuer   string
}

// NewJWTAuthenticator creates a JWT authenticator that fetches signing keys
// from jwksURL. It performs an initial key fetch to fail fast at startup if
// the OIDC provider is unreachable.
func NewJWTAuthenticator(ctx context.Context, jwksURL, audience, issuer string) (*JWTAuthenticator, error) {
	cache := jwk.NewCache(ctx)

	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("registering JWKS URL: %w", err)
	}

	// Fail fast: if the provider is unreachable at startup, surface the error
	// immediately rather than silently returning 401 to every request.
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch from %s: %w", jwksURL, err)
	}

	return &JWTAuthenticator{
		cache:    cache,
		jwksURL:  jwksURL,
		audience: audience,
		issuer:   issuer,
	}, nil
}

// Authenticate parses and validates a raw JWT string. It verifies the
// signature against the cached JWKS, checks standard claims (exp, iss, aud),
// and extracts OAD-specific custom claims (oad_roles, oad_system_id).
func (a *JWTAuthenticator) Authenticate(ctx context.Context, tokenString string) (*Identity, error) {
	keySet, err := a.cache.Get(ctx, a.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}

	opts := []jwt.ParseOption{
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(a.issuer),
		jwt.WithAudience(a.audience),
	}

	token, err := jwt.Parse([]byte(tokenString), opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	sub := token.Subject()
	if sub == "" {
		return nil, errors.New("token missing required 'sub' claim")
	}

	roles, err := extractStringSlice(token, "oad_roles")
	if err != nil {
		return nil, err
	}

	systemID, _ := extractString(token, "oad_system_id")

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
