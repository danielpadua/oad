package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/auth"
)

// MeResponse is the JSON shape returned by GET /api/v1/me.
type MeResponse struct {
	Sub             string   `json:"sub"`
	Provider        string   `json:"provider"`
	EntityID        string   `json:"entity_id"`
	Groups          []string `json:"groups"`
	IsPlatformAdmin bool     `json:"is_platform_admin"`
	AllowedSystems  []string `json:"allowed_systems"`
}

// MeHandler serves GET /api/v1/me.
type MeHandler struct{}

// NewMeHandler creates a MeHandler.
func NewMeHandler() *MeHandler { return &MeHandler{} }

// Get returns the authenticated caller's resolved identity.
func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	identity := auth.MustIdentityFromContext(r.Context())

	allowedSystems := make([]string, len(identity.AllowedSystems))
	for i, s := range identity.AllowedSystems {
		allowedSystems[i] = s.String()
	}

	groups := identity.Groups
	if groups == nil {
		groups = []string{}
	}

	entityID := ""
	if identity.EntityID != (uuid.UUID{}) {
		entityID = identity.EntityID.String()
	}

	resp := MeResponse{
		Sub:             identity.Subject,
		Provider:        identity.Provider,
		EntityID:        entityID,
		Groups:          groups,
		IsPlatformAdmin: identity.IsPlatformAdmin,
		AllowedSystems:  allowedSystems,
	}
	response.JSON(w, http.StatusOK, resp)
}
