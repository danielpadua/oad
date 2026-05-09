package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/auth"
)

// Authentication returns middleware that validates caller credentials and
// stores the resulting Identity in the request context.
func Authentication(
	jwtAuth *auth.JWTAuthenticator,
	mtlsAuth *auth.MTLSAuthenticator,
	cache *auth.IdentityCache,
	mode string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var identity *auth.Identity
			var err error

			switch mode {
			case "jwt":
				identity, err = resolveJWT(r, jwtAuth, cache)
			case "mtls":
				identity, err = mtlsAuth.Authenticate(r)
			case "both":
				identity, err = resolveJWT(r, jwtAuth, cache)
				if err != nil && mtlsAuth != nil {
					identity, err = mtlsAuth.Authenticate(r)
				}
			default:
				response.Error(w, apierr.Internal("unsupported auth mode: "+mode))
				return
			}

			if err != nil {
				if errors.Is(err, auth.ErrNotProvisioned) {
					response.Error(w, apierr.Unauthorized("account not yet provisioned — try again in a moment"))
					return
				}
				if errors.Is(err, auth.ErrDisabled) {
					response.Error(w, apierr.Unauthorized("account is disabled"))
					return
				}
				slog.WarnContext(r.Context(), "authentication failed", "err", err)
				response.Error(w, apierr.Unauthorized("authentication failed"))
				return
			}

			// Parse X-OAD-System-Id header and set ActiveSystemID on the identity.
			if headerVal := r.Header.Get("X-OAD-System-Id"); headerVal != "" {
				systemID, parseErr := uuid.Parse(headerVal)
				if parseErr != nil {
					response.Error(w, apierr.BadRequest("X-OAD-System-Id must be a valid UUID"))
					return
				}
				if !identity.IsPlatformAdmin && !systemInAllowed(systemID, identity.AllowedSystems) {
					response.Error(w, apierr.Forbidden("X-OAD-System-Id not in allowed systems"))
					return
				}
				identity.ActiveSystemID = &systemID
			}

			ctx := auth.WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveJWT(r *http.Request, jwtAuth *auth.JWTAuthenticator, cache *auth.IdentityCache) (*auth.Identity, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, errMissingAuth
	}
	token, found := strings.CutPrefix(header, "Bearer ")
	if !found {
		return nil, errMissingAuth
	}
	raw, err := jwtAuth.AuthenticateRaw(r.Context(), token)
	if err != nil {
		return nil, err
	}
	return cache.Resolve(r.Context(), raw.Provider, raw.Subject)
}

func systemInAllowed(id uuid.UUID, allowed []uuid.UUID) bool {
	for _, a := range allowed {
		if a == id {
			return true
		}
	}
	return false
}

var errMissingAuth = &authError{msg: "missing or malformed Authorization header"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
