package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Provider configures a single trusted Identity Provider.
type Provider struct {
	Name     string // config name, e.g. "keycloak" — returned in RawToken.Provider
	JWKSURL  string
	Issuer   string
	Audience string
}

// RawToken is the result of JWT validation before DB identity resolution.
// It carries only the stable identity claims; all authorization attributes
// are resolved from the DB by IdentityResolver.
type RawToken struct {
	Provider string // matched Provider.Name
	Subject  string // JWT "sub" claim
}

// JWTAuthenticator validates JWT Bearer tokens using per-provider JWKS endpoints.
type JWTAuthenticator struct {
	cache     *jwk.Cache
	providers map[string]Provider // keyed by Issuer
}

// NewJWTAuthenticator creates a JWT authenticator for one or more trusted
// identity providers. It performs an initial key fetch per provider to fail
// fast at startup if any OIDC provider is unreachable.
func NewJWTAuthenticator(ctx context.Context, providers []Provider) (*JWTAuthenticator, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	cache := jwk.NewCache(ctx)
	byIssuer := make(map[string]Provider, len(providers))

	for _, p := range providers {
		if p.Name == "" {
			return nil, fmt.Errorf("provider with issuer %q has empty name", p.Issuer)
		}
		if err := cache.Register(p.JWKSURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
			return nil, fmt.Errorf("registering JWKS URL %q: %w", p.JWKSURL, err)
		}
		if _, err := cache.Refresh(ctx, p.JWKSURL); err != nil {
			return nil, fmt.Errorf("initial JWKS fetch from %s: %w", p.JWKSURL, err)
		}
		byIssuer[p.Issuer] = p
	}

	return &JWTAuthenticator{cache: cache, providers: byIssuer}, nil
}

// AuthenticateRaw parses and validates a raw JWT string, returning only
// the provider name and subject claim. The caller is responsible for
// resolving the full Identity from the DB via IdentityResolver.
func (a *JWTAuthenticator) AuthenticateRaw(ctx context.Context, tokenString string) (RawToken, error) {
	raw, err := jwt.ParseInsecure([]byte(tokenString))
	if err != nil {
		return RawToken{}, fmt.Errorf("malformed token: %w", err)
	}

	p, ok := a.providers[raw.Issuer()]
	if !ok {
		return RawToken{}, fmt.Errorf("untrusted token issuer %q", raw.Issuer())
	}

	keySet, err := a.cache.Get(ctx, p.JWKSURL)
	if err != nil {
		return RawToken{}, fmt.Errorf("fetching JWKS for issuer %q: %w", p.Issuer, err)
	}

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(p.Issuer),
		jwt.WithAudience(p.Audience),
	)
	if err != nil {
		return RawToken{}, fmt.Errorf("invalid token: %w", err)
	}

	sub := token.Subject()
	if sub == "" {
		return RawToken{}, errors.New("token missing required 'sub' claim")
	}

	return RawToken{Provider: p.Name, Subject: sub}, nil
}
