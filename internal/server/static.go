package server

import (
	"io/fs"
	"net/http"
	"strings"

	webui "github.com/lattapon-aek/agent-orchestrator-management/web"
)

// spaHandler serves the embedded frontend SPA.
// Rules:
//   - Paths starting with /api/ or /ws/ are NOT handled here (routed to REST/WS handlers).
//   - Paths that match a real file in dist/ (e.g. /assets/...) are served directly.
//   - Everything else returns index.html so React Router handles client-side navigation.
type spaHandler struct {
	fs http.FileSystem
}

func newSPAHandler() http.Handler {
	// Strip the leading "dist/" prefix so requests for /assets/foo.js
	// map to dist/assets/foo.js inside the embedded FS.
	sub, err := fs.Sub(webui.FS, "dist")
	if err != nil {
		panic("webui: embed.FS missing dist/ directory: " + err.Error())
	}
	return &spaHandler{fs: http.FS(sub)}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Delegate API and WebSocket paths to other handlers — never serve them as files.
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/ws/") {
		http.NotFound(w, r)
		return
	}

	// Try to open the requested path as a real asset.
	f, err := h.fs.Open(p)
	if err != nil {
		// Not a real file → serve index.html for SPA client-side routing.
		// no-store so browsers always fetch the latest index.html (and thus the
		// latest content-hashed JS/CSS bundles).
		w.Header().Set("Cache-Control", "no-store")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(h.fs).ServeHTTP(w, r2)
		return
	}
	f.Close()

	// Hashed assets (/assets/index-XYZ.js) are immutable — cache forever.
	// index.html itself must never be cached.
	if strings.HasPrefix(p, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.FileServer(h.fs).ServeHTTP(w, r)
}
