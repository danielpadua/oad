// Package handler implements the SCIM 2.0 HTTP surface. Phase 9.B.2
// covers the discovery endpoints only:
//
//	GET /scim/v2/ServiceProviderConfig — capability advertisement
//	GET /scim/v2/Schemas               — schema introspection (empty in B.2)
//	GET /scim/v2/ResourceTypes         — resource type listing (empty in B.2)
//
// Schemas and ResourceTypes return empty lists until the User and Group
// CRUD endpoints land in Phase 9.B.3 / 9.B.4.
package handler

import (
	"net/http"

	"github.com/danielpadua/oad/internal/scim/response"
	"github.com/danielpadua/oad/internal/scim/schema"
)

const (
	scimURNListResponse           = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimURNServiceProviderConfig  = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	scimMaxFilterResults          = 200
	scimServiceProviderConfigPath = "/scim/v2/ServiceProviderConfig"
)

// DiscoveryHandler serves the three SCIM 2.0 discovery endpoints. It carries
// no state of its own; all responses are static at the protocol level.
type DiscoveryHandler struct{}

// NewDiscoveryHandler returns a ready-to-use DiscoveryHandler.
func NewDiscoveryHandler() *DiscoveryHandler {
	return &DiscoveryHandler{}
}

// supportedFlag is the {"supported": bool} shape used by ServiceProviderConfig
// for non-parameterized capabilities (sort, changePassword, ...).
type supportedFlag struct {
	Supported bool `json:"supported"`
}

type filterCapability struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type bulkCapability struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

type authScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     bool   `json:"primary"`
}

type meta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location"`
}

type serviceProviderConfig struct {
	Schemas               []string         `json:"schemas"`
	Patch                 supportedFlag    `json:"patch"`
	Bulk                  bulkCapability   `json:"bulk"`
	Filter                filterCapability `json:"filter"`
	ChangePassword        supportedFlag    `json:"changePassword"`
	Sort                  supportedFlag    `json:"sort"`
	ETag                  supportedFlag    `json:"etag"`
	AuthenticationSchemes []authScheme     `json:"authenticationSchemes"`
	Meta                  meta             `json:"meta"`
}

type listResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	Resources    []any    `json:"Resources"`
}

// ServiceProviderConfig handles GET /scim/v2/ServiceProviderConfig.
// Advertises which SCIM optional features OAD implements. Filter and ETag
// are advertised as supported in anticipation of B.3 onward; bulk, sort,
// and changePassword are out of scope for the foreseeable future.
func (h *DiscoveryHandler) ServiceProviderConfig(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, serviceProviderConfig{
		Schemas:        []string{scimURNServiceProviderConfig},
		Patch:          supportedFlag{Supported: true},
		Bulk:           bulkCapability{Supported: false},
		Filter:         filterCapability{Supported: true, MaxResults: scimMaxFilterResults},
		ChangePassword: supportedFlag{Supported: false},
		Sort:           supportedFlag{Supported: false},
		ETag:           supportedFlag{Supported: true},
		AuthenticationSchemes: []authScheme{{
			Type:        "oauthbearertoken",
			Name:        "OAuth Bearer Token",
			Description: "Per-provider SCIM tenant token configured via auth.providers[].scim.token",
			Primary:     true,
		}},
		Meta: meta{
			ResourceType: "ServiceProviderConfig",
			Location:     scimServiceProviderConfigPath,
		},
	})
}

// Schemas handles GET /scim/v2/Schemas. Returns the list of schema
// definitions the server supports. Phase 9.B.3 added the User schema;
// Group will be added in 9.B.4.
func (h *DiscoveryHandler) Schemas(w http.ResponseWriter, _ *http.Request) {
	resources := []any{schema.User()}
	response.WriteJSON(w, http.StatusOK, listResponse{
		Schemas:      []string{scimURNListResponse},
		TotalResults: len(resources),
		Resources:    resources,
	})
}

// ResourceTypes handles GET /scim/v2/ResourceTypes. Returns the list of
// resource types the server supports. Phase 9.B.3 added User; Group will
// be added in 9.B.4.
func (h *DiscoveryHandler) ResourceTypes(w http.ResponseWriter, _ *http.Request) {
	resources := []any{schema.UserResourceType()}
	response.WriteJSON(w, http.StatusOK, listResponse{
		Schemas:      []string{scimURNListResponse},
		TotalResults: len(resources),
		Resources:    resources,
	})
}
