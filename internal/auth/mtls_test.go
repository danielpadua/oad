package auth

import (
	"testing"
)

func TestBuildMTLSIdentity_NilOUs(t *testing.T) {
	id := buildMTLSIdentity("cn-service", nil)
	if id.IsPlatformAdmin {
		t.Error("nil OUs must not set IsPlatformAdmin")
	}
	if len(id.Groups) != 0 {
		t.Errorf("nil OUs must yield empty Groups, got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_AdminOU(t *testing.T) {
	id := buildMTLSIdentity("cn-admin", []string{"admin"})
	if !id.IsPlatformAdmin {
		t.Error("admin OU must set IsPlatformAdmin")
	}
	if len(id.Groups) != 0 {
		t.Errorf("admin OU must not add groups, got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_EditorOU(t *testing.T) {
	id := buildMTLSIdentity("cn-editor", []string{"editor"})
	if id.IsPlatformAdmin {
		t.Error("editor OU must not set IsPlatformAdmin")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "oad:editor" {
		t.Errorf("editor OU must yield Groups=[oad:editor], got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_ViewerOU(t *testing.T) {
	id := buildMTLSIdentity("cn-viewer", []string{"viewer"})
	if id.IsPlatformAdmin {
		t.Error("viewer OU must not set IsPlatformAdmin")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "oad:viewer" {
		t.Errorf("viewer OU must yield Groups=[oad:viewer], got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_AdminPlusEditor(t *testing.T) {
	id := buildMTLSIdentity("cn-power", []string{"admin", "editor"})
	if !id.IsPlatformAdmin {
		t.Error("admin OU must set IsPlatformAdmin")
	}
	if len(id.Groups) != 1 || id.Groups[0] != "oad:editor" {
		t.Errorf("editor OU must yield Groups=[oad:editor], got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_UnknownOUsIgnored(t *testing.T) {
	id := buildMTLSIdentity("cn-unknown", []string{"superuser", "root", "unknown"})
	if id.IsPlatformAdmin {
		t.Error("unknown OUs must not set IsPlatformAdmin")
	}
	if len(id.Groups) != 0 {
		t.Errorf("unknown OUs must not add groups, got %v", id.Groups)
	}
}

func TestBuildMTLSIdentity_ProviderAndAuthMode(t *testing.T) {
	id := buildMTLSIdentity("cn-svc", []string{"viewer"})
	if id.Provider != "mtls" {
		t.Errorf("Provider = %q, want %q", id.Provider, "mtls")
	}
	if id.AuthMode != "mtls" {
		t.Errorf("AuthMode = %q, want %q", id.AuthMode, "mtls")
	}
}
