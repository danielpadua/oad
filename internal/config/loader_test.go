package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/danielpadua/oad/internal/config"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "oad-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoad_RejectsObsoleteClaimsMapping(t *testing.T) {
	yamlContent := `
auth:
  mode: jwt
  providers:
    - name: idp
      backend:
        jwks_url: https://idp.test/.well-known/jwks
        issuer: https://idp.test
        audience: oad-api
        claims_mapping:
          roles_claim: groups
database:
  dsn: postgresql://localhost/oad
`
	f := writeTempConfig(t, yamlContent)
	_, err := config.Load(config.CLIOptions{ConfigFile: f})
	if err == nil {
		t.Fatal("expected error for obsolete claims_mapping, got nil")
	}
	if !strings.Contains(err.Error(), "claims_mapping") {
		t.Errorf("error should mention claims_mapping, got: %v", err)
	}
}
