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

func TestJWTAuthenticator_ValidToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	srv, kid := testJWKS(t, &privKey.PublicKey)

	ctx := context.Background()
	authn, err := auth.NewJWTAuthenticator(ctx, srv.URL, "oad-api", "https://idp.example.com")
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	token := signToken(t, privKey, kid, map[string]any{
		"sub":           "user@example.com",
		"iss":           "https://idp.example.com",
		"aud":           []string{"oad-api"},
		"exp":           time.Now().Add(time.Hour).Unix(),
		"iat":           time.Now().Unix(),
		"oad_roles":     []any{"admin", "editor"},
		"oad_system_id": "550e8400-e29b-41d4-a716-446655440000",
	})

	identity, err := authn.Authenticate(ctx, token)
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

	ctx := context.Background()
	authn, err := auth.NewJWTAuthenticator(ctx, srv.URL, "oad-api", "https://idp.example.com")
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	token := signToken(t, privKey, kid, map[string]any{
		"sub": "user@example.com",
		"iss": "https://idp.example.com",
		"aud": []string{"oad-api"},
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err = authn.Authenticate(ctx, token)
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

	ctx := context.Background()
	authn, err := auth.NewJWTAuthenticator(ctx, srv.URL, "oad-api", "https://idp.example.com")
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	token := signToken(t, privKey, kid, map[string]any{
		"sub": "user@example.com",
		"iss": "https://idp.example.com",
		"aud": []string{"wrong-audience"},
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err = authn.Authenticate(ctx, token)
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

	ctx := context.Background()
	authn, err := auth.NewJWTAuthenticator(ctx, srv.URL, "oad-api", "https://idp.example.com")
	if err != nil {
		t.Fatalf("creating authenticator: %v", err)
	}

	token := signToken(t, privKey, kid, map[string]any{
		"sub":       "admin@example.com",
		"iss":       "https://idp.example.com",
		"aud":       []string{"oad-api"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"oad_roles": []any{"admin"},
	})

	identity, err := authn.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.SystemID != "" {
		t.Errorf("expected empty system_id for platform admin, got %s", identity.SystemID)
	}
}
