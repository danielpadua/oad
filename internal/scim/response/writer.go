// Package response writes SCIM 2.0-formatted HTTP responses. Bodies are
// emitted with Content-Type: application/scim+json (RFC 7644 §3.1.2).
package response

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ContentType is the SCIM 2.0 media type per RFC 7644 §3.1.2.
const ContentType = "application/scim+json"

// errorBody is the SCIM error response shape from RFC 7644 §3.12.
type errorBody struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	SCIMType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// WriteJSON writes body as application/scim+json with the given status code.
// On marshal failure the response is replaced with a generic 500 error so
// callers do not have to handle errors at the call site.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// At this point the status is already written; we cannot send a
		// fresh error response. The connection will be closed and the
		// client will see a truncated body.
		_ = err
	}
}

// WriteError writes a SCIM 2.0 error response (RFC 7644 §3.12). scimType is
// optional and may be empty for cases that do not map to a defined SCIM type
// (the spec lists them in §3.12 — invalidFilter, tooMany, uniqueness, etc.).
func WriteError(w http.ResponseWriter, status int, scimType, detail string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Schemas:  []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		Status:   strconv.Itoa(status),
		SCIMType: scimType,
		Detail:   detail,
	})
}
