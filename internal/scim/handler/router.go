package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/danielpadua/oad/internal/scim/auth"
)

// Mount attaches the SCIM 2.0 endpoints to r at the /scim/v2 prefix.
//
// Discovery endpoints (ServiceProviderConfig, Schemas, ResourceTypes) are
// unauthenticated per RFC 7644 §4.
//
// The registry argument is consumed by the auth middleware that protects
// resource endpoints. Phase 9.B.2 registers no resource endpoints, so the
// registry is currently used only to keep the wiring symmetric with the
// rest of the codebase: B.3 (Users) attaches the auth middleware to the
// /Users sub-route group inside this same Mount call.
func Mount(r chi.Router, registry *auth.Registry) {
	disc := NewDiscoveryHandler()

	r.Route("/scim/v2", func(r chi.Router) {
		r.Get("/ServiceProviderConfig", disc.ServiceProviderConfig)
		r.Get("/Schemas", disc.Schemas)
		r.Get("/ResourceTypes", disc.ResourceTypes)

		// Resource endpoints (Users, Groups) attach inside an authenticated
		// group in Phase 9.B.3 / 9.B.4:
		//
		//   r.Group(func(r chi.Router) {
		//       r.Use(auth.Authenticate(registry))
		//       // /Users, /Groups routes
		//   })
		_ = registry
	})
}
