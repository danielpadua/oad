// Package main implements a lightweight JWKS stub server for local development.
//
// It generates an RSA keypair at startup and exposes two HTTP endpoints:
//
//   - GET  /.well-known/jwks.json — serves the public key set (JWKS)
//   - POST /token                 — mints signed JWTs with caller-defined claims
//   - GET  /health                — simple liveness check
//
// This replaces the need for a full-blown IdP (Keycloak, Auth0, etc.) during
// development, while remaining compatible with the production JWT validation
// path in internal/auth.
//
// All configuration is via environment variables:
//
//	JWKS_PORT       — listen port (default: 9090)
//	JWT_ISSUER      — "iss" claim value (default: http://localhost:<port>)
//	JWT_AUDIENCE    — "aud" claim value (default: oad-api)
//	JWT_TTL         — default token lifetime (default: 1h)
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// tokenRequest defines the JSON body accepted by POST /token.
type tokenRequest struct {
	Sub       string   `json:"sub"`
	Roles     []string `json:"oad_roles"`
	SystemID  string   `json:"oad_system_id,omitempty"`
	ExpiresIn string   `json:"expires_in,omitempty"` // Go duration string, e.g. "2h", "30m"
}

func main() {
	port := getEnv("JWKS_PORT", "9090")
	issuer := getEnv("JWT_ISSUER", "http://localhost:"+port)
	audience := getEnv("JWT_AUDIENCE", "oad-api")
	defaultTTL := getEnvDuration("JWT_TTL", time.Hour)
	kid := "dev-key-1"

	// -----------------------------------------------------------------
	// Generate an ephemeral RSA keypair — new keys every restart.
	// This is intentional: dev tokens should never outlive the server.
	// -----------------------------------------------------------------
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		slog.Error("generating RSA key", "error", err)
		os.Exit(1)
	}

	// Build the JWKS (public key set) that the API will fetch.
	pubJWK, err := jwk.FromRaw(&privKey.PublicKey)
	if err != nil {
		slog.Error("creating public JWK", "error", err)
		os.Exit(1)
	}
	_ = pubJWK.Set(jwk.KeyIDKey, kid)
	_ = pubJWK.Set(jwk.AlgorithmKey, jwa.RS256)
	_ = pubJWK.Set(jwk.KeyUsageKey, "sig")

	keySet := jwk.NewSet()
	_ = keySet.AddKey(pubJWK)

	// Private JWK for signing tokens.
	privJWK, err := jwk.FromRaw(privKey)
	if err != nil {
		slog.Error("creating private JWK", "error", err)
		os.Exit(1)
	}
	_ = privJWK.Set(jwk.KeyIDKey, kid)

	// -----------------------------------------------------------------
	// HTTP Handlers
	// -----------------------------------------------------------------
	mux := http.NewServeMux()

	// GET /.well-known/jwks.json — serves the public JWKS.
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=900")
		if err := json.NewEncoder(w).Encode(keySet); err != nil {
			slog.Error("encoding JWKS", "error", err)
		}
	})

	// POST /token — mints a signed JWT with the provided claims.
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		var req tokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		if req.Sub == "" {
			writeJSONError(w, http.StatusBadRequest, "\"sub\" is required")
			return
		}

		ttl := defaultTTL
		if req.ExpiresIn != "" {
			if d, err := time.ParseDuration(req.ExpiresIn); err == nil {
				ttl = d
			} else {
				writeJSONError(w, http.StatusBadRequest, "invalid expires_in duration: "+err.Error())
				return
			}
		}

		now := time.Now()
		tok := jwt.New()
		_ = tok.Set(jwt.SubjectKey, req.Sub)
		_ = tok.Set(jwt.IssuerKey, issuer)
		_ = tok.Set(jwt.AudienceKey, audience)
		_ = tok.Set(jwt.IssuedAtKey, now)
		_ = tok.Set(jwt.ExpirationKey, now.Add(ttl))

		if len(req.Roles) > 0 {
			_ = tok.Set("oad_roles", req.Roles)
		}
		if req.SystemID != "" {
			_ = tok.Set("oad_system_id", req.SystemID)
		}

		signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, privJWK))
		if err != nil {
			slog.Error("signing token", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "signing failed")
			return
		}

		slog.Info("token issued",
			"sub", req.Sub,
			"roles", req.Roles,
			"system_id", req.SystemID,
			"ttl", ttl.String(),
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      string(signed),
			"expires_in": int(ttl.Seconds()),
			"token_type": "Bearer",
		})
	})

	// GET /health — liveness probe for docker-compose healthcheck.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// -----------------------------------------------------------------
	// Start server
	// -----------------------------------------------------------------
	addr := ":" + port
	slog.Info("jwks-stub server starting",
		"addr", addr,
		"issuer", issuer,
		"audience", audience,
		"default_ttl", defaultTTL.String(),
		"endpoints", map[string]string{
			"jwks":   fmt.Sprintf("http://localhost:%s/.well-known/jwks.json", port),
			"token":  fmt.Sprintf("http://localhost:%s/token", port),
			"health": fmt.Sprintf("http://localhost:%s/health", port),
		},
	)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// writeJSONError writes a JSON error response with the given status code.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
