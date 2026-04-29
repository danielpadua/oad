package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/danielpadua/oad/internal/auth"
)

// testJWKS starts an HTTP server serving a JWKS endpoint with the given
// RSA public key and returns the server and key ID.
func testJWKS(t *testing.T, pub *rsa.PublicKey) (srv *httptest.Server, kid string) {
	t.Helper()
	kid = "test-key-1"

	key, err := jwk.FromRaw(pub)
	if err != nil {
		t.Fatalf("creating JWK from public key: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("setting kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("setting alg: %v", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(key); err != nil {
		t.Fatalf("adding key to set: %v", err)
	}

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)

	return srv, kid
}

func signToken(t *testing.T, privKey *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	builder := jwt.New()
	for k, v := range claims {
		if err := builder.Set(k, v); err != nil {
			t.Fatalf("setting claim %q: %v", k, err)
		}
	}

	key, err := jwk.FromRaw(privKey)
	if err != nil {
		t.Fatalf("creating JWK from private key: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		t.Fatalf("setting kid: %v", err)
	}

	signed, err := jwt.Sign(builder, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return string(signed)
}

func newSingleProviderAuthn(t *testing.T, jwksURL, issuer, audience string) *auth.JWTAuthenticator {
	t.Helper()
	authn, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{JWKSURL: jwksURL, Issuer: issuer, Audience: audience},
	})
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}
	return authn
}

func TestJWTAuthenticator_ValidToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)
	authn := newSingleProviderAuthn(t, srv.URL, "https://idp.example.com", "oad-api")

	token := signToken(t, privKey, kid, map[string]any{
		"sub":           "user@example.com",
		"iss":           "https://idp.example.com",
		"aud":           []string{"oad-api"},
		"exp":           time.Now().Add(time.Hour).Unix(),
		"iat":           time.Now().Unix(),
		"oad_roles":     []any{"admin", "editor"},
		"oad_system_id": "550e8400-e29b-41d4-a716-446655440000",
	})

	identity, err := authn.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.Subject != "user@example.com" {
		t.Errorf("expected subject user@example.com, got %s", identity.Subject)
	}
	if !identity.HasRole("admin") || !identity.HasRole("editor") {
		t.Errorf("expected roles [admin, editor], got %v", identity.Roles)
	}
	if identity.SystemID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected system_id 550e8400-..., got %s", identity.SystemID)
	}
	if identity.AuthMode != "jwt" {
		t.Errorf("expected auth_mode jwt, got %s", identity.AuthMode)
	}
}

func TestJWTAuthenticator_ExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)
	authn := newSingleProviderAuthn(t, srv.URL, "https://idp.example.com", "oad-api")

	token := signToken(t, privKey, kid, map[string]any{
		"sub": "user@example.com",
		"iss": "https://idp.example.com",
		"aud": []string{"oad-api"},
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err = authn.Authenticate(context.Background(), token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWTAuthenticator_WrongAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)
	authn := newSingleProviderAuthn(t, srv.URL, "https://idp.example.com", "oad-api")

	token := signToken(t, privKey, kid, map[string]any{
		"sub": "user@example.com",
		"iss": "https://idp.example.com",
		"aud": []string{"wrong-audience"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = authn.Authenticate(context.Background(), token)
	if err == nil {
		t.Error("expected error for wrong audience")
	}
}

func TestJWTAuthenticator_PlatformAdmin_NoSystemID(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)
	authn := newSingleProviderAuthn(t, srv.URL, "https://idp.example.com", "oad-api")

	token := signToken(t, privKey, kid, map[string]any{
		"sub":       "admin@example.com",
		"iss":       "https://idp.example.com",
		"aud":       []string{"oad-api"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"oad_roles": []any{"admin"},
	})

	identity, err := authn.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.SystemID != "" {
		t.Errorf("expected empty system_id for platform admin, got %s", identity.SystemID)
	}
}

func TestJWTAuthenticator_UntrustedIssuer(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)
	authn := newSingleProviderAuthn(t, srv.URL, "https://idp.example.com", "oad-api")

	token := signToken(t, privKey, kid, map[string]any{
		"sub": "attacker@evil.com",
		"iss": "https://evil.com",
		"aud": []string{"oad-api"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = authn.Authenticate(context.Background(), token)
	if err == nil {
		t.Error("expected error for untrusted issuer")
	}
}

func TestJWTAuthenticator_ClaimsMapping_CustomRolesClaim(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)

	authn, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{
			JWKSURL:  srv.URL,
			Issuer:   "https://dex.example.com",
			Audience: "oad-web-dex",
			ClaimsMapping: auth.ClaimsMapping{
				RolesClaim: "groups",
			},
		},
	})
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	token := signToken(t, privKey, kid, map[string]any{
		"sub":    "user@dex.example.com",
		"iss":    "https://dex.example.com",
		"aud":    []string{"oad-web-dex"},
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []any{"editor"},
	})

	identity, err := authn.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !identity.HasRole("editor") {
		t.Errorf("expected role editor from groups claim, got %v", identity.Roles)
	}
}

func TestJWTAuthenticator_ClaimsMapping_DefaultRoles(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)

	authn, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{
			JWKSURL:  srv.URL,
			Issuer:   "https://dex.example.com",
			Audience: "oad-web-dex",
			ClaimsMapping: auth.ClaimsMapping{
				RolesClaim:   "groups",
				DefaultRoles: []string{"viewer"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	// Token with no groups claim — default_roles must apply.
	token := signToken(t, privKey, kid, map[string]any{
		"sub": "user@dex.example.com",
		"iss": "https://dex.example.com",
		"aud": []string{"oad-web-dex"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	identity, err := authn.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !identity.HasRole("viewer") {
		t.Errorf("expected default role viewer, got %v", identity.Roles)
	}
}

func TestJWTAuthenticator_ClaimsMapping_CustomSystemIDClaim(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)

	authn, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{
			JWKSURL:  srv.URL,
			Issuer:   "https://idp.example.com",
			Audience: "oad-api",
			ClaimsMapping: auth.ClaimsMapping{
				SystemIDClaim: "x_system_id",
			},
		},
	})
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	const wantSystemID = "550e8400-e29b-41d4-a716-446655440000"
	token := signToken(t, privKey, kid, map[string]any{
		"sub":         "svc@example.com",
		"iss":         "https://idp.example.com",
		"aud":         []string{"oad-api"},
		"exp":         time.Now().Add(time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"oad_roles":   []any{"viewer"},
		"x_system_id": wantSystemID,
	})

	identity, err := authn.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.SystemID != wantSystemID {
		t.Errorf("expected system_id %s from x_system_id claim, got %q", wantSystemID, identity.SystemID)
	}
}

func TestJWTAuthenticator_MultiProvider_PerAudience(t *testing.T) {
	privKeyA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key A: %v", err)
	}
	privKeyB, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key B: %v", err)
	}

	srvA, kidA := testJWKS(t, &privKeyA.PublicKey)
	srvB, kidB := testJWKS(t, &privKeyB.PublicKey)

	authn, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{JWKSURL: srvA.URL, Issuer: "https://idp-a.example.com", Audience: "audience-a"},
		{JWKSURL: srvB.URL, Issuer: "https://idp-b.example.com", Audience: "audience-b"},
	})
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	// Token from provider A with audience-a must be accepted.
	tokenA := signToken(t, privKeyA, kidA, map[string]any{
		"sub": "user-a@example.com",
		"iss": "https://idp-a.example.com",
		"aud": []string{"audience-a"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	idA, err := authn.Authenticate(context.Background(), tokenA)
	if err != nil {
		t.Fatalf("provider A token rejected: %v", err)
	}
	if idA.Subject != "user-a@example.com" {
		t.Errorf("expected user-a, got %s", idA.Subject)
	}

	// Token from provider B with audience-b must be accepted.
	tokenB := signToken(t, privKeyB, kidB, map[string]any{
		"sub": "user-b@example.com",
		"iss": "https://idp-b.example.com",
		"aud": []string{"audience-b"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	idB, err := authn.Authenticate(context.Background(), tokenB)
	if err != nil {
		t.Fatalf("provider B token rejected: %v", err)
	}
	if idB.Subject != "user-b@example.com" {
		t.Errorf("expected user-b, got %s", idB.Subject)
	}

	// Token from provider A with audience-b must be rejected (wrong audience for issuer).
	tokenAWrongAud := signToken(t, privKeyA, kidA, map[string]any{
		"sub": "user-a@example.com",
		"iss": "https://idp-a.example.com",
		"aud": []string{"audience-b"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	_, err = authn.Authenticate(context.Background(), tokenAWrongAud)
	if err == nil {
		t.Error("expected rejection for provider-A token with audience-b")
	}
}
