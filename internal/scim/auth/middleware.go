package auth

import (
	"net/http"
	"strings"

	"github.com/danielpadua/oad/internal/scim/response"
)

const bearerPrefix = "Bearer "

// Authenticate is HTTP middleware that requires a registered SCIM tenant
// token. On success, the resolved provider name is injected into the request
// context (retrievable via ProviderFromContext). On failure, a SCIM-formatted
// 401 error is written and downstream handlers are not invoked.
func Authenticate(registry *Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(authHeader, bearerPrefix)
			if !ok {
				response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "missing or malformed Authorization header; expected 'Bearer <token>'")
				return
			}
			provider, ok := registry.Lookup(token)
			if !ok {
				response.WriteError(w, http.StatusUnauthorized, "invalidAuth", "bearer token not recognized")
				return
			}
			ctx := WithProvider(r.Context(), provider)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
