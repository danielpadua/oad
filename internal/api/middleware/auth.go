package middleware

import (
	"net/http"
	"strings"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/auth"
)

// Authentication returns middleware that validates caller credentials and
// stores the resulting Identity in the request context. It supports JWT
// Bearer tokens, mTLS client certificates, or both (try JWT first, fall
// back to mTLS). Unauthenticated requests receive a 401 response.
//
// Either jwtAuth or mtlsAuth may be nil when the corresponding mode is
// not enabled. The mode parameter controls which authenticators are tried:
// "jwt", "mtls", or "both".
func Authentication(jwtAuth *auth.JWTAuthenticator, mtlsAuth *auth.MTLSAuthenticator, mode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var identity *auth.Identity
			var err error

			switch mode {
			case "jwt":
				identity, err = authenticateJWT(jwtAuth, r)
			case "mtls":
				identity, err = mtlsAuth.Authenticate(r)
			case "both":
				// Try JWT first (most common); fall back to mTLS.
				identity, err = authenticateJWT(jwtAuth, r)
				if err != nil && mtlsAuth != nil {
					identity, err = mtlsAuth.Authenticate(r)
				}
			default:
				response.Error(w, apierr.Internal("unsupported auth mode: "+mode))
				return
			}

			if err != nil {
				response.Error(w, apierr.Unauthorized("authentication failed: "+err.Error()))
				return
			}

			ctx := auth.WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateJWT extracts the Bearer token from the Authorization header
// and delegates validation to the JWTAuthenticator.
func authenticateJWT(jwtAuth *auth.JWTAuthenticator, r *http.Request) (*auth.Identity, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, errMissingAuth
	}

	token, found := strings.CutPrefix(header, "Bearer ")
	if !found {
		return nil, errMissingAuth
	}

	return jwtAuth.Authenticate(r.Context(), token)
}

var errMissingAuth = &authError{msg: "missing or malformed Authorization header"}

type authError struct {
	msg string
}

func (e *authError) Error() string { return e.msg }
