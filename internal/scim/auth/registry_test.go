package auth_test

import (
	"errors"
	"testing"

	"github.com/danielpadua/oad/internal/scim/auth"
)

func TestRegistry_Register_ValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider string
		token    string
		wantErr  error
	}{
		{"empty provider", "", "secret", auth.ErrEmptyProvider},
		{"empty token", "keycloak", "", auth.ErrEmptyToken},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := auth.NewRegistry()
			err := r.Register(tc.provider, tc.token)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Register(%q, %q) error = %v, want %v", tc.provider, tc.token, err, tc.wantErr)
			}
			if r.Size() != 0 {
				t.Errorf("Size() after failed Register = %d, want 0", r.Size())
			}
		})
	}
}

func TestRegistry_Register_DuplicateToken(t *testing.T) {
	t.Parallel()

	r := auth.NewRegistry()
	if err := r.Register("keycloak", "shared-secret"); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register("authentik", "shared-secret")
	if !errors.Is(err, auth.ErrDuplicateToken) {
		t.Fatalf("second Register error = %v, want ErrDuplicateToken", err)
	}
	if r.Size() != 1 {
		t.Errorf("Size() = %d, want 1 (duplicate must not be added)", r.Size())
	}
}

func TestRegistry_Lookup(t *testing.T) {
	t.Parallel()

	r := auth.NewRegistry()
	if err := r.Register("keycloak", "kc-token"); err != nil {
		t.Fatalf("Register keycloak: %v", err)
	}
	if err := r.Register("authentik", "ak-token"); err != nil {
		t.Fatalf("Register authentik: %v", err)
	}

	cases := []struct {
		name         string
		token        string
		wantProvider string
		wantOK       bool
	}{
		{"matches keycloak", "kc-token", "keycloak", true},
		{"matches authentik", "ak-token", "authentik", true},
		{"unknown token", "intruder", "", false},
		{"empty token", "", "", false},
		{"case sensitive", "KC-TOKEN", "", false},
		{"prefix is not a match", "kc-token-extra", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, ok := r.Lookup(tc.token)
			if ok != tc.wantOK {
				t.Errorf("Lookup(%q) ok = %v, want %v", tc.token, ok, tc.wantOK)
			}
			if provider != tc.wantProvider {
				t.Errorf("Lookup(%q) provider = %q, want %q", tc.token, provider, tc.wantProvider)
			}
		})
	}
}

func TestRegistry_Size(t *testing.T) {
	t.Parallel()

	r := auth.NewRegistry()
	if r.Size() != 0 {
		t.Fatalf("empty Size = %d, want 0", r.Size())
	}
	_ = r.Register("p1", "t1")
	_ = r.Register("p2", "t2")
	_ = r.Register("p3", "t3")
	if r.Size() != 3 {
		t.Errorf("Size after 3 Registers = %d, want 3", r.Size())
	}
}
