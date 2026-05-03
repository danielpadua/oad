package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/danielpadua/oad/internal/scim/auth"
)

// Handlers groups the resource-specific SCIM handlers passed to Mount.
// Discovery is constructed by Mount when nil; resource handlers (Users,
// future Groups) are optional — when omitted the corresponding route
// group is not registered and clients receive 404 from Chi's default.
type Handlers struct {
	Discovery *DiscoveryHandler
	Users     *UsersHandler
}

// Mount attaches the SCIM 2.0 endpoints to r at the /scim/v2 prefix.
//
// Discovery endpoints (ServiceProviderConfig, Schemas, ResourceTypes) are
// unauthenticated per RFC 7644 §4. Resource endpoints (Users, future
// Groups) sit inside an authenticated group that consults the SCIM tenant
// token registry on every request.
func Mount(r chi.Router, registry *auth.Registry, h Handlers) {
	if h.Discovery == nil {
		h.Discovery = NewDiscoveryHandler()
	}

	r.Route("/scim/v2", func(r chi.Router) {
		r.Get("/ServiceProviderConfig", h.Discovery.ServiceProviderConfig)
		r.Get("/Schemas", h.Discovery.Schemas)
		r.Get("/ResourceTypes", h.Discovery.ResourceTypes)

		if h.Users != nil {
			r.Group(func(r chi.Router) {
				r.Use(auth.Authenticate(registry))
				r.Get("/Users", h.Users.List)
				r.Post("/Users", h.Users.Create)
				r.Get("/Users/{id}", h.Users.GetByID)
				r.Put("/Users/{id}", h.Users.Replace)
				r.Delete("/Users/{id}", h.Users.Delete)
			})
		}
	})
}
