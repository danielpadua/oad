package handler

import (
	"net/http"

	"github.com/danielpadua/oad/internal/api/response"
	"github.com/danielpadua/oad/internal/config"
)

// ConfigHandler serves the frontend OIDC configuration at /config.json.
// Only WebUI-facing fields are exposed; backend fields (jwks_url, issuer,
// audience) are never sent to the browser.
type ConfigHandler struct {
	cfg *config.Config
}

func NewConfigHandler(cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{cfg: cfg}
}

type configJSONResponse struct {
	Providers     []configProviderResponse `json:"providers"`
	RedirectURI   string                   `json:"redirect_uri"`
	PostLogoutURI string                   `json:"post_logout_uri"`
}

type configProviderResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Authority   string `json:"authority"`
	ClientID    string `json:"client_id"`
	Scope       string `json:"scope"`
}

// Get writes the frontend OIDC configuration as JSON.
// The response is intentionally public (no auth required) so the
// browser can bootstrap its OIDC client before any login.
func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	providers := make([]configProviderResponse, 0, len(h.cfg.Auth.Providers))
	for _, p := range h.cfg.Auth.Providers {
		providers = append(providers, configProviderResponse{
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Authority:   p.WebUI.Authority,
			ClientID:    p.WebUI.ClientID,
			Scope:       p.WebUI.Scope,
		})
	}

	response.JSON(w, http.StatusOK, configJSONResponse{
		Providers:     providers,
		RedirectURI:   h.cfg.WebUI.RedirectURI,
		PostLogoutURI: h.cfg.WebUI.PostLogoutURI,
	})
}
