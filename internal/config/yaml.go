package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// fileConfig mirrors the YAML file structure exactly.
// All fields are pointers or have zero values so the loader can distinguish
// "not set in file" from "explicitly set to zero/empty".
type fileConfig struct {
	Server   fileServerConfig   `yaml:"server"`
	Database fileDatabaseConfig `yaml:"database"`
	Auth     fileAuthConfig     `yaml:"auth"`
	WebUI    fileWebUIConfig    `yaml:"webui"`
	Log      fileLogConfig      `yaml:"log"`
}

type fileServerConfig struct {
	Addr            string        `yaml:"addr"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type fileDatabaseConfig struct {
	DSN      string `yaml:"dsn"`
	MaxConns int32  `yaml:"max_conns"`
	MinConns int32  `yaml:"min_conns"`
}

type fileAuthConfig struct {
	Mode       string               `yaml:"mode"`
	MTLSHeader string               `yaml:"mtls_header"`
	Providers  []fileProviderConfig `yaml:"providers"`
}

type fileProviderConfig struct {
	Name        string             `yaml:"name"`
	DisplayName string             `yaml:"display_name"`
	Backend     fileProviderBackend `yaml:"backend"`
	WebUI       fileProviderWebUI  `yaml:"webui"`
}

type fileProviderBackend struct {
	JWKSURL       string             `yaml:"jwks_url"`
	Issuer        string             `yaml:"issuer"`
	Audience      string             `yaml:"audience"`
	ClaimsMapping fileClaimsMapping  `yaml:"claims_mapping"`
}

type fileClaimsMapping struct {
	RolesClaim    string   `yaml:"roles_claim"`
	SystemIDClaim string   `yaml:"system_id_claim"`
	DefaultRoles  []string `yaml:"default_roles"`
}

type fileProviderWebUI struct {
	Authority string `yaml:"authority"`
	ClientID  string `yaml:"client_id"`
	Scope     string `yaml:"scope"`
}

type fileWebUIConfig struct {
	RedirectURI   string `yaml:"redirect_uri"`
	PostLogoutURI string `yaml:"post_logout_uri"`
}

type fileLogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func loadFile(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return &fc, nil
}
