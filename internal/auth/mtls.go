package auth

import (
	"errors"
	"net/http"
)

// MTLSAuthenticator extracts identity from a client certificate presented
// during mutual TLS. When TLS is terminated at a load balancer, the LB
// forwards the client certificate in a configurable HTTP header.
type MTLSAuthenticator struct {
	headerName string // e.g., "X-Client-Cert"
}

// NewMTLSAuthenticator creates an authenticator that checks for a client
// certificate in the TLS connection state first, then falls back to the
// given header name for LB-terminated TLS deployments.
func NewMTLSAuthenticator(headerName string) *MTLSAuthenticator {
	return &MTLSAuthenticator{headerName: headerName}
}

// Authenticate extracts the caller identity from the client certificate.
// It checks the direct TLS peer certificates first; if none are present
// (LB-terminated TLS), it reads the certificate CN from the configured header.
//
// For MVP, the CN becomes the Subject, and the identity receives no roles
// or system scope — those must be mapped externally or via a lookup table
// in a future iteration.
func (a *MTLSAuthenticator) Authenticate(r *http.Request) (*Identity, error) {
	// Direct TLS termination: peer certificates are available.
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		cert := r.TLS.PeerCertificates[0]
		return &Identity{
			Subject:  cert.Subject.CommonName,
			Roles:    extractRolesFromCert(cert.Subject.OrganizationalUnit),
			AuthMode: "mtls",
		}, nil
	}

	// LB-terminated TLS: the load balancer forwards the client CN in a header.
	cn := r.Header.Get(a.headerName)
	if cn == "" {
		return nil, errors.New("no client certificate presented")
	}

	return &Identity{
		Subject:  cn,
		AuthMode: "mtls",
	}, nil
}

// extractRolesFromCert maps certificate OU fields to OAD roles.
// OUs matching known role names are included; others are ignored.
func extractRolesFromCert(ous []string) []string {
	validRoles := map[string]bool{"admin": true, "editor": true, "viewer": true}
	var roles []string
	for _, ou := range ous {
		if validRoles[ou] {
			roles = append(roles, ou)
		}
	}
	return roles
}
