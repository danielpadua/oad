package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/apierr"
	"github.com/danielpadua/oad/internal/auth"
)

// RequireRole returns middleware that rejects requests where the authenticated
// identity does not hold the specified role. Returns 403 Forbidden on failure.
// Must be chained after Authentication middleware.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}
			if !identity.HasRole(role) {
				response.Error(w, apierr.Forbidden("role '"+role+"' required"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole returns middleware that accepts if the identity holds at
// least one of the listed roles. Returns 403 Forbidden when none match.
func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}
			if !identity.HasAnyRole(roles...) {
				response.Error(w, apierr.Forbidden("insufficient role"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlatformAdmin returns middleware that rejects requests from identities
// bound to a specific system scope. Only unscoped identities (SystemID == "") —
// platform administrators — may pass. Must be chained after Authentication.
func RequirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			response.Error(w, apierr.Unauthorized("missing identity"))
			return
		}
		if identity.SystemID != "" {
			response.Error(w, apierr.Forbidden("platform admin required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireSystemScope returns middleware that verifies the caller is authorized
// for the system identified by the given URL parameter. Platform admins
// (empty SystemID) bypass the check — they have access to all systems.
func RequireSystemScope(paramName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}

			// Platform admins have unrestricted system access.
			if identity.SystemID == "" {
				next.ServeHTTP(w, r)
				return
			}

			requested := chi.URLParam(r, paramName)
			if requested != "" && requested != identity.SystemID {
				response.Error(w, apierr.Forbidden("access denied to system "+requested))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
