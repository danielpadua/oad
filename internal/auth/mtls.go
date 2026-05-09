package auth

import (
	"errors"
	"net/http"
)

// MTLSAuthenticator extracts identity from a client certificate.
type MTLSAuthenticator struct {
	headerName string
}

// NewMTLSAuthenticator creates an authenticator that checks for a client
// certificate in the TLS connection state first, then falls back to the
// given header name for LB-terminated TLS deployments.
func NewMTLSAuthenticator(headerName string) *MTLSAuthenticator {
	return &MTLSAuthenticator{headerName: headerName}
}

// Authenticate returns an Identity from the client certificate. mTLS callers
// are not resolved through the DB; roles derive from certificate OU fields
// mapped to built-in group external IDs.
func (a *MTLSAuthenticator) Authenticate(r *http.Request) (*Identity, error) {
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		cert := r.TLS.PeerCertificates[0]
		return buildMTLSIdentity(cert.Subject.CommonName, cert.Subject.OrganizationalUnit), nil
	}

	cn := r.Header.Get(a.headerName)
	if cn == "" {
		return nil, errors.New("no client certificate presented")
	}
	return buildMTLSIdentity(cn, nil), nil
}

// buildMTLSIdentity synthesises an Identity from a certificate CN and OUs.
// OUs are mapped to built-in group external IDs:
//
//	"admin"  → IsPlatformAdmin = true
//	"editor" → Groups includes "oad:editor"
//	"viewer" → Groups includes "oad:viewer"
func buildMTLSIdentity(cn string, ous []string) *Identity {
	id := &Identity{
		Subject:  cn,
		Provider: "mtls",
		AuthMode: "mtls",
	}
	for _, ou := range ous {
		switch ou {
		case "admin":
			id.IsPlatformAdmin = true
		case "editor":
			id.Groups = append(id.Groups, "oad:editor")
		case "viewer":
			id.Groups = append(id.Groups, "oad:viewer")
		}
	}
	return id
}
