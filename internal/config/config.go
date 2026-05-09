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
	Mode              string
	MTLSHeader        string
	Providers         []ProviderConfig
	BootstrapAdmins   []BootstrapAdmin
	IdentityCacheTTL  time.Duration // default 30s
	IdentityCacheSize int           // default 10000
}

// BootstrapAdmin seeds a platform admin user on startup.
// The entry must match the JWT (provider, sub) pair for the user to log in.
type BootstrapAdmin struct {
	Provider string
	Subject  string
}

// ProviderConfig represents a single trusted Identity Provider.
// Backend carries the fields used by the API to validate tokens.
// WebUI carries the fields served to the frontend via /config.json.
// SCIM carries the tenant token used by the IdP to authenticate against
// OAD's /scim/v2 endpoints (Phase 9.B).
type ProviderConfig struct {
	Name        string
	DisplayName string
	Backend     ProviderBackend
	WebUI       ProviderWebUI
	SCIM        ProviderSCIM
}

// ProviderSCIM holds the SCIM ingest configuration for one provider.
// Token is the resolved plaintext bearer token; the loader resolves
// `env:VAR_NAME` references before returning. The token is consumed once
// at startup to populate the SCIM auth registry; it is not retained on
// the Config struct after that point in production paths.
type ProviderSCIM struct {
	Enabled bool
	Token   string
}

type ProviderBackend struct {
	JWKSURL  string
	Issuer   string
	Audience string
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
