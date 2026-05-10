package middleware

import (
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
// that are not platform admins. Only identities with IsPlatformAdmin == true
// may pass. Must be chained after Authentication.
func RequirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			response.Error(w, apierr.Unauthorized("missing identity"))
			return
		}
		if !identity.IsPlatformAdmin {
			response.Error(w, apierr.Forbidden("platform admin required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePathSystemInScope returns middleware that verifies the UUID in the
// given URL path parameter is within the caller's AllowedSystems.
// Platform admins bypass the check. Returns 403 if the system is not allowed,
// 400 if the path parameter is not a valid UUID.
// Must be chained after Authentication middleware.
func RequirePathSystemInScope(pathParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}
			rawID := chi.URLParam(r, pathParam)
			systemID, err := uuid.Parse(rawID)
			if err != nil {
				response.Error(w, apierr.BadRequest(pathParam+" must be a valid UUID"))
				return
			}
			if identity.IsPlatformAdmin {
				next.ServeHTTP(w, r)
				return
			}
			if slices.Contains(identity.AllowedSystems, systemID) {
				next.ServeHTTP(w, r)
				return
			}
			response.Error(w, apierr.Forbidden("access denied to system "+rawID))
		})
	}
}

// RequireSystemScope requires the X-OAD-System-Id header to have been set
// and validated by the Authentication middleware. Platform admins must also
// send the header to pin an active system when accessing scoped resources.
func RequireSystemScope(_ string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok {
				response.Error(w, apierr.Unauthorized("missing identity"))
				return
			}
			if identity.ActiveSystemID == nil {
				response.Error(w, apierr.BadRequest("X-OAD-System-Id header required for this endpoint"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
