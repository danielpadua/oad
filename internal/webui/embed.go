// Package webui embeds the compiled Management UI (web/dist → internal/webui/dist)
// into the binary and exposes an http.Handler that serves it as a SPA.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var staticFS embed.FS

// NewHandler returns an http.Handler that serves the embedded SPA.
//
// Exact file matches (JS, CSS, assets) are served as-is.
// Any unrecognised path falls through to index.html so React Router can
// handle client-side navigation without 404s on hard refreshes.
func NewHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "dist")
	if err != nil {
		return nil, err
	}
	return &spaHandler{
		fileServer: http.FileServer(http.FS(sub)),
		staticFS:   sub,
	}, nil
}

type spaHandler struct {
	fileServer http.Handler
	staticFS   fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Derive the lookup name: strip the leading slash and clean the path.
	// An empty result (root "/") maps to "index.html".
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if name == "." {
		name = "index.html"
	}

	if _, err := h.staticFS.Open(name); err != nil {
		// File not found — rewrite to root so http.FileServer returns index.html,
		// which lets React Router handle the path client-side.
		r = r.Clone(r.Context())
		r.URL.Path = "/"
	}

	h.fileServer.ServeHTTP(w, r)
}
