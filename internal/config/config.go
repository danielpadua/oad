// Package config loads and validates OAD runtime configuration.
// Precedence (highest to lowest): CLI flag → OAD_* env var → YAML file → default.
package config

import "time"

// Config holds the fully-resolved runtime configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	WebUI    WebUIConfig
	Log      LogConfig
}

type ServerConfig struct {
	Addr            string        // [host]:port — default ":8080"
	ShutdownTimeout time.Duration // default 30s
}

type DatabaseConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

// AuthConfig controls authentication for incoming requests.
// Mode selects accepted credential types: "jwt", "mtls", "both", or "none".
// Providers defines the set of trusted Identity Providers (for jwt / both modes).
type AuthConfig struct {
	Mode       string
	MTLSHeader string
	Providers  []ProviderConfig
}

// ProviderConfig represents a single trusted Identity Provider.
// Backend carries the fields used by the API to validate tokens.
// WebUI carries the fields served to the frontend via /config.json.
type ProviderConfig struct {
	Name        string
	DisplayName string
	Backend     ProviderBackend
	WebUI       ProviderWebUI
}

type ProviderBackend struct {
	JWKSURL       string
	Issuer        string
	Audience      string
	ClaimsMapping ClaimsMapping
}

// ClaimsMapping adapts an IdP's native token claims to OAD's identity model.
// Leave fields empty to use OAD's defaults (oad_roles, oad_system_id).
type ClaimsMapping struct {
	// RolesClaim is the JWT claim that carries the user's roles.
	// Defaults to "oad_roles" when empty.
	RolesClaim string
	// SystemIDClaim is the JWT claim for the scoped system UUID.
	// Defaults to "oad_system_id" when empty.
	SystemIDClaim string
	// DefaultRoles are assigned when RolesClaim is absent from the token.
	// Useful for IdPs that don't emit per-user role claims (e.g. Dex with
	// staticPasswords, which doesn't support a groups claim).
	DefaultRoles []string
}

type ProviderWebUI struct {
	Authority string
	ClientID  string
	Scope     string
}

// WebUIConfig holds configuration served to the frontend at /config.json.
// redirect_uri and post_logout_uri are per-application, shared across providers.
type WebUIConfig struct {
	RedirectURI   string
	PostLogoutURI string
}

type LogConfig struct {
	Level  string // debug | info | warn | error  — default "info"
	Format string // json | text                   — default "json"
}

// CLIOptions carries values supplied via cobra flags.
// An empty string means "not set by the caller"; the loader skips it.
type CLIOptions struct {
	ConfigFile      string
	Database        string
	Addr            string
	AuthMode        string
	ShutdownTimeout string
	LogLevel        string
	LogFormat       string
}

// Load resolves configuration by merging CLI flags, OAD_* env vars,
// an optional YAML file, and built-in defaults.
func Load(opts CLIOptions) (*Config, error) {
	return load(opts)
}
