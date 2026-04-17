package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
// No defaults contain secrets — all sensitive values are required explicitly.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Host            string
	Port            int
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

// AuthConfig controls how the API authenticates incoming requests.
// Mode selects which credential types are accepted: "jwt", "mtls", or "both".
type AuthConfig struct {
	Mode        string   // AUTH_MODE: "jwt" (default), "mtls", or "both"
	JWKSURLs    []string // JWKS_URL: required when mode includes jwt (comma-separated)
	JWTAudience string   // JWT_AUDIENCE: expected "aud" claim
	JWTIssuers  []string // JWT_ISSUER: expected "iss" claim (comma-separated)
	MTLSHeader  string   // MTLS_HEADER: header name for LB-terminated mTLS (default: X-Client-Cert)
}

// Load reads configuration from environment variables.
// Returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	authMode := getEnvStr("AUTH_MODE", "jwt")

	authCfg := AuthConfig{
		Mode:        authMode,
		JWKSURLs:    getEnvStrSlice("JWKS_URL"),
		JWTAudience: os.Getenv("JWT_AUDIENCE"),
		JWTIssuers:  getEnvStrSlice("JWT_ISSUER"),
		MTLSHeader:  getEnvStr("MTLS_HEADER", "X-Client-Cert"),
	}

	if authMode == "jwt" || authMode == "both" {
		if len(authCfg.JWKSURLs) == 0 {
			return nil, fmt.Errorf("JWKS_URL is required when AUTH_MODE is %q", authMode)
		}
		if authCfg.JWTAudience == "" {
			return nil, fmt.Errorf("JWT_AUDIENCE is required when AUTH_MODE is %q", authMode)
		}
		if len(authCfg.JWTIssuers) == 0 {
			return nil, fmt.Errorf("JWT_ISSUER is required when AUTH_MODE is %q", authMode)
		}
	}

	return &Config{
		Server: ServerConfig{
			Host:            getEnvStr("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			URL:      dbURL,
			MaxConns: getEnvInt32("DB_MAX_CONNS", 25),
			MinConns: getEnvInt32("DB_MIN_CONNS", 5),
		},
		Auth: authCfg,
	}, nil
}

func getEnvStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvInt32(key string, defaultVal int32) int32 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(i)
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvStrSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	var result []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
