package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// load merges CLI flags → OAD_* env vars → YAML file → defaults.
func load(opts CLIOptions) (*Config, error) {
	var fc *fileConfig
	if opts.ConfigFile != "" {
		var err error
		fc, err = loadFile(opts.ConfigFile)
		if err != nil {
			return nil, err
		}
	}

	warnDeprecated()

	cfg := &Config{}
	applyDefaults(cfg)

	if fc != nil {
		applyFile(cfg, fc)
	}

	applyEnv(cfg)
	applyFlags(cfg, opts)

	return cfg, validate(cfg)
}

// applyDefaults sets built-in defaults on a zero-value Config.
func applyDefaults(cfg *Config) {
	cfg.Server.Addr = ":8080"
	cfg.Server.ShutdownTimeout = 30 * time.Second
	cfg.Database.MaxConns = 25
	cfg.Database.MinConns = 5
	cfg.Auth.Mode = "jwt"
	cfg.Auth.MTLSHeader = "X-Client-Cert"
	cfg.Log.Level = "info"
	cfg.Log.Format = "json"
}

// applyFile overlays non-zero values from the parsed YAML file.
func applyFile(cfg *Config, fc *fileConfig) {
	if fc.Server.Addr != "" {
		cfg.Server.Addr = fc.Server.Addr
	}
	if fc.Server.ShutdownTimeout != 0 {
		cfg.Server.ShutdownTimeout = fc.Server.ShutdownTimeout
	}
	if fc.Database.DSN != "" {
		cfg.Database.URL = fc.Database.DSN
	}
	if fc.Database.MaxConns != 0 {
		cfg.Database.MaxConns = fc.Database.MaxConns
	}
	if fc.Database.MinConns != 0 {
		cfg.Database.MinConns = fc.Database.MinConns
	}
	if fc.Auth.Mode != "" {
		cfg.Auth.Mode = fc.Auth.Mode
	}
	if fc.Auth.MTLSHeader != "" {
		cfg.Auth.MTLSHeader = fc.Auth.MTLSHeader
	}
	for _, fp := range fc.Auth.Providers {
		cfg.Auth.Providers = append(cfg.Auth.Providers, ProviderConfig{
			Name:        fp.Name,
			DisplayName: fp.DisplayName,
			Backend: ProviderBackend{
				JWKSURL:  fp.Backend.JWKSURL,
				Issuer:   fp.Backend.Issuer,
				Audience: fp.Backend.Audience,
				ClaimsMapping: ClaimsMapping{
					RolesClaim:    fp.Backend.ClaimsMapping.RolesClaim,
					SystemIDClaim: fp.Backend.ClaimsMapping.SystemIDClaim,
					DefaultRoles:  fp.Backend.ClaimsMapping.DefaultRoles,
				},
			},
			WebUI: ProviderWebUI{
				Authority: fp.WebUI.Authority,
				ClientID:  fp.WebUI.ClientID,
				Scope:     fp.WebUI.Scope,
			},
		})
	}
	if fc.WebUI.RedirectURI != "" {
		cfg.WebUI.RedirectURI = fc.WebUI.RedirectURI
	}
	if fc.WebUI.PostLogoutURI != "" {
		cfg.WebUI.PostLogoutURI = fc.WebUI.PostLogoutURI
	}
	if fc.Log.Level != "" {
		cfg.Log.Level = fc.Log.Level
	}
	if fc.Log.Format != "" {
		cfg.Log.Format = fc.Log.Format
	}
}

// applyEnv overlays values from OAD_* environment variables.
// Flat provider env vars (OAD_JWKS_URL, OAD_JWT_ISSUER, OAD_JWT_AUDIENCE)
// build or replace an anonymous single-provider entry — useful when a full
// YAML file is not needed.
func applyEnv(cfg *Config) {
	if v := os.Getenv("OAD_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("OAD_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ShutdownTimeout = d
		}
	}
	if v := os.Getenv("OAD_DATABASE"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("OAD_DB_MAX_CONNS"); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.Database.MaxConns = int32(i)
		}
	}
	if v := os.Getenv("OAD_DB_MIN_CONNS"); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.Database.MinConns = int32(i)
		}
	}
	if v := os.Getenv("OAD_AUTH_MODE"); v != "" {
		cfg.Auth.Mode = v
	}
	if v := os.Getenv("OAD_MTLS_HEADER"); v != "" {
		cfg.Auth.MTLSHeader = v
	}
	if v := os.Getenv("OAD_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("OAD_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}

	// Flat single-provider env vars: build/replace the first provider entry.
	// Covers both backend (JWT validation) and WebUI (frontend OIDC client) fields
	// so that a single-IdP deployment can be fully configured without a YAML file.
	// The embedded dev IdP (devidp build tag) uses this mechanism to self-register.
	{
		jwksURL := os.Getenv("OAD_JWKS_URL")
		issuer := os.Getenv("OAD_JWT_ISSUER")
		audience := os.Getenv("OAD_JWT_AUDIENCE")
		name := os.Getenv("OAD_PROVIDER_NAME")
		displayName := os.Getenv("OAD_PROVIDER_DISPLAY_NAME")
		authority := os.Getenv("OAD_PROVIDER_AUTHORITY")
		clientID := os.Getenv("OAD_PROVIDER_CLIENT_ID")
		scope := os.Getenv("OAD_PROVIDER_SCOPE")

		if jwksURL != "" || issuer != "" || audience != "" ||
			name != "" || displayName != "" ||
			authority != "" || clientID != "" || scope != "" {
			p := ProviderConfig{Name: "default"}
			if len(cfg.Auth.Providers) > 0 {
				p = cfg.Auth.Providers[0]
			}
			if name != "" {
				p.Name = name
			}
			if displayName != "" {
				p.DisplayName = displayName
			}
			if jwksURL != "" {
				p.Backend.JWKSURL = jwksURL
			}
			if issuer != "" {
				p.Backend.Issuer = issuer
			}
			if audience != "" {
				p.Backend.Audience = audience
			}
			if authority != "" {
				p.WebUI.Authority = authority
			}
			if clientID != "" {
				p.WebUI.ClientID = clientID
			}
			if scope != "" {
				p.WebUI.Scope = scope
			}
			if len(cfg.Auth.Providers) == 0 {
				cfg.Auth.Providers = []ProviderConfig{p}
			} else {
				cfg.Auth.Providers[0] = p
			}
		}
	}

	if v := os.Getenv("OAD_WEBUI_REDIRECT_URI"); v != "" {
		cfg.WebUI.RedirectURI = v
	}
	if v := os.Getenv("OAD_WEBUI_POST_LOGOUT_URI"); v != "" {
		cfg.WebUI.PostLogoutURI = v
	}
}

// applyFlags overlays non-empty CLI flag values (highest precedence).
func applyFlags(cfg *Config, opts CLIOptions) {
	if opts.Database != "" {
		cfg.Database.URL = opts.Database
	}
	if opts.Addr != "" {
		cfg.Server.Addr = opts.Addr
	}
	if opts.AuthMode != "" {
		cfg.Auth.Mode = opts.AuthMode
	}
	if opts.ShutdownTimeout != "" {
		if d, err := time.ParseDuration(opts.ShutdownTimeout); err == nil {
			cfg.Server.ShutdownTimeout = d
		}
	}
	if opts.LogLevel != "" {
		cfg.Log.Level = opts.LogLevel
	}
	if opts.LogFormat != "" {
		cfg.Log.Format = opts.LogFormat
	}
}

// validate checks that the resolved Config is coherent.
func validate(cfg *Config) error {
	if cfg.Database.URL == "" {
		return fmt.Errorf("database DSN is required (--database, OAD_DATABASE, or database.dsn in config file)")
	}

	mode := cfg.Auth.Mode
	if mode != "jwt" && mode != "mtls" && mode != "both" && mode != "none" {
		return fmt.Errorf("invalid auth mode %q: must be jwt, mtls, both, or none", mode)
	}

	if mode == "jwt" || mode == "both" {
		if len(cfg.Auth.Providers) == 0 {
			return fmt.Errorf("auth mode %q requires at least one provider (configure via YAML auth.providers or OAD_JWKS_URL/OAD_JWT_ISSUER/OAD_JWT_AUDIENCE)", mode)
		}
		for i, p := range cfg.Auth.Providers {
			if p.Backend.JWKSURL == "" {
				return fmt.Errorf("provider[%d] %q: backend.jwks_url is required", i, p.Name)
			}
			if p.Backend.Issuer == "" {
				return fmt.Errorf("provider[%d] %q: backend.issuer is required", i, p.Name)
			}
			if p.Backend.Audience == "" {
				return fmt.Errorf("provider[%d] %q: backend.audience is required", i, p.Name)
			}
		}
	}

	logFormat := cfg.Log.Format
	if logFormat != "json" && logFormat != "text" {
		return fmt.Errorf("invalid log format %q: must be json or text", logFormat)
	}

	return nil
}

// warnDeprecated logs a warning for each legacy env var that is still set.
var legacyEnvVars = []struct {
	old string
	new string
}{
	{"DATABASE_URL", "OAD_DATABASE"},
	{"SERVER_HOST", "OAD_ADDR (combined host:port)"},
	{"SERVER_PORT", "OAD_ADDR (combined host:port)"},
	{"SERVER_SHUTDOWN_TIMEOUT", "OAD_SHUTDOWN_TIMEOUT"},
	{"AUTH_MODE", "OAD_AUTH_MODE"},
	{"JWKS_URL", "OAD_JWKS_URL or auth.providers in config file"},
	{"JWT_AUDIENCE", "OAD_JWT_AUDIENCE or auth.providers in config file"},
	{"JWT_ISSUER", "OAD_JWT_ISSUER or auth.providers in config file"},
	{"MTLS_HEADER", "OAD_MTLS_HEADER"},
	{"DB_MAX_CONNS", "OAD_DB_MAX_CONNS"},
	{"DB_MIN_CONNS", "OAD_DB_MIN_CONNS"},
}

func warnDeprecated() {
	for _, e := range legacyEnvVars {
		if os.Getenv(e.old) != "" {
			slog.Warn("deprecated env var detected — please migrate",
				"deprecated", e.old,
				"use_instead", e.new,
			)
		}
	}
	// Legacy support: if old vars are set and new OAD_* are not, copy values.
	copyLegacyEnv("DATABASE_URL", "OAD_DATABASE")
	copyLegacyEnv("AUTH_MODE", "OAD_AUTH_MODE")
	copyLegacyEnv("MTLS_HEADER", "OAD_MTLS_HEADER")
	copyLegacyEnv("SERVER_SHUTDOWN_TIMEOUT", "OAD_SHUTDOWN_TIMEOUT")
	copyLegacyEnv("DB_MAX_CONNS", "OAD_DB_MAX_CONNS")
	copyLegacyEnv("DB_MIN_CONNS", "OAD_DB_MIN_CONNS")

	// SERVER_HOST + SERVER_PORT → OAD_ADDR
	if os.Getenv("OAD_ADDR") == "" {
		host := os.Getenv("SERVER_HOST")
		port := os.Getenv("SERVER_PORT")
		if host != "" || port != "" {
			if host == "" {
				host = "0.0.0.0"
			}
			if port == "" {
				port = "8080"
			}
			os.Setenv("OAD_ADDR", host+":"+port) //nolint:errcheck
		}
	}

	// JWKS_URL is comma-separated; map to OAD_JWKS_URL (same format).
	copyLegacyEnvComma("JWKS_URL", "OAD_JWKS_URL")
	copyLegacyEnvComma("JWT_ISSUER", "OAD_JWT_ISSUER")
	copyLegacyEnv("JWT_AUDIENCE", "OAD_JWT_AUDIENCE")
}

func copyLegacyEnv(old, new string) {
	if os.Getenv(old) != "" && os.Getenv(new) == "" {
		os.Setenv(new, os.Getenv(old)) //nolint:errcheck
	}
}

// copyLegacyEnvComma copies old → new only for single-value cases.
// Multi-value JWKS_URL / JWT_ISSUER picks up the first entry only.
func copyLegacyEnvComma(old, new string) {
	if os.Getenv(old) != "" && os.Getenv(new) == "" {
		parts := strings.SplitN(os.Getenv(old), ",", 2)
		os.Setenv(new, strings.TrimSpace(parts[0])) //nolint:errcheck
	}
}
