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

func generateTestKey(t *testing.T) (*rsa.PrivateKey, jwk.Key) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubKey, err := jwk.PublicKeyOf(priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	if err := pubKey.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatal(err)
	}
	return priv, pubKey
}

func jwksServer(t *testing.T, pubKey jwk.Key) *httptest.Server {
	t.Helper()
	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw) //nolint:errcheck // test helper; response write error is irrelevant
	}))
}

func signToken(t *testing.T, priv *rsa.PrivateKey, issuer, audience, sub string, extra map[string]any) string {
	t.Helper()
	b := jwt.NewBuilder().
		Issuer(issuer).
		Audience([]string{audience}).
		Subject(sub).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Hour))
	for k, v := range extra {
		b.Claim(k, v)
	}
	tok, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	// Embed kid so the JWKS key lookup matches.
	privJWK, err := jwk.FromRaw(priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := privJWK.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, privJWK))
	if err != nil {
		t.Fatal(err)
	}
	return string(signed)
}

func TestJWTAuthenticator_AuthenticateRaw_ValidToken(t *testing.T) {
	priv, pubKey := generateTestKey(t)
	srv := jwksServer(t, pubKey)
	defer srv.Close()

	a, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{Name: "test-idp", JWKSURL: srv.URL, Issuer: "https://idp.test", Audience: "oad-api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signToken(t, priv, "https://idp.test", "oad-api", "user-123", nil)
	raw, err := a.AuthenticateRaw(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw.Subject != "user-123" {
		t.Errorf("subject = %q, want %q", raw.Subject, "user-123")
	}
	if raw.Provider != "test-idp" {
		t.Errorf("provider = %q, want %q", raw.Provider, "test-idp")
	}
}

func TestJWTAuthenticator_AuthenticateRaw_IgnoresClaims(t *testing.T) {
	priv, pubKey := generateTestKey(t)
	srv := jwksServer(t, pubKey)
	defer srv.Close()

	a, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{Name: "test-idp", JWKSURL: srv.URL, Issuer: "https://idp.test", Audience: "oad-api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Token with legacy claims — must be accepted without reading them.
	token := signToken(t, priv, "https://idp.test", "oad-api", "user-456", map[string]any{
		"oad_roles":     []string{"admin"},
		"oad_system_id": "some-uuid",
	})
	raw, err := a.AuthenticateRaw(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw.Subject != "user-456" {
		t.Errorf("subject = %q, want %q", raw.Subject, "user-456")
	}
}

func TestJWTAuthenticator_AuthenticateRaw_UnknownIssuer(t *testing.T) {
	priv, pubKey := generateTestKey(t)
	srv := jwksServer(t, pubKey)
	defer srv.Close()

	a, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{Name: "test-idp", JWKSURL: srv.URL, Issuer: "https://idp.test", Audience: "oad-api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signToken(t, priv, "https://evil.test", "oad-api", "attacker", nil)
	_, err = a.AuthenticateRaw(context.Background(), token)
	if err == nil {
		t.Error("expected error for unknown issuer, got nil")
	}
}

func TestJWTAuthenticator_AuthenticateRaw_ExpiredToken(t *testing.T) {
	priv, pubKey := generateTestKey(t)
	srv := jwksServer(t, pubKey)
	defer srv.Close()

	a, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{Name: "test-idp", JWKSURL: srv.URL, Issuer: "https://idp.test", Audience: "oad-api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Build an already-expired token manually so we can control the expiry.
	b := jwt.NewBuilder().
		Issuer("https://idp.test").
		Audience([]string{"oad-api"}).
		Subject("user-expired").
		IssuedAt(time.Now().Add(-2 * time.Hour)).
		Expiration(time.Now().Add(-time.Hour))
	tok, buildErr := b.Build()
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	privJWK, jwkErr := jwk.FromRaw(priv)
	if jwkErr != nil {
		t.Fatal(jwkErr)
	}
	if setErr := privJWK.Set(jwk.KeyIDKey, "test-key"); setErr != nil {
		t.Fatal(setErr)
	}
	signed, signErr := jwt.Sign(tok, jwt.WithKey(jwa.RS256, privJWK))
	if signErr != nil {
		t.Fatal(signErr)
	}

	_, err = a.AuthenticateRaw(context.Background(), string(signed))
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestJWTAuthenticator_AuthenticateRaw_MultiProvider(t *testing.T) {
	privA, pubA := generateTestKey(t)
	privB, pubB := generateTestKey(t)
	srvA := jwksServer(t, pubA)
	srvB := jwksServer(t, pubB)
	defer srvA.Close()
	defer srvB.Close()

	a, err := auth.NewJWTAuthenticator(context.Background(), []auth.Provider{
		{Name: "idp-a", JWKSURL: srvA.URL, Issuer: "https://idp-a.test", Audience: "aud-a"},
		{Name: "idp-b", JWKSURL: srvB.URL, Issuer: "https://idp-b.test", Audience: "aud-b"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenA := signToken(t, privA, "https://idp-a.test", "aud-a", "user-a", nil)
	rawA, err := a.AuthenticateRaw(context.Background(), tokenA)
	if err != nil {
		t.Fatalf("provider A token rejected: %v", err)
	}
	if rawA.Provider != "idp-a" || rawA.Subject != "user-a" {
		t.Errorf("provider A: got provider=%q subject=%q", rawA.Provider, rawA.Subject)
	}

	tokenB := signToken(t, privB, "https://idp-b.test", "aud-b", "user-b", nil)
	rawB, err := a.AuthenticateRaw(context.Background(), tokenB)
	if err != nil {
		t.Fatalf("provider B token rejected: %v", err)
	}
	if rawB.Provider != "idp-b" || rawB.Subject != "user-b" {
		t.Errorf("provider B: got provider=%q subject=%q", rawB.Provider, rawB.Subject)
	}
}
