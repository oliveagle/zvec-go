package server

import (
	_ "embed"
	"net/http"
)

// indexHTML is the embedded single-page web UI (server/web/index.html).
//
//go:embed web/index.html
var indexHTML []byte

// handleUI serves the embedded web console.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(indexHTML)
}

// handleRoot is the catch-all: API paths return a JSON 404, any other path
// serves the web UI (so opening "/" or "/ui" in a browser shows the console).
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	s.handleUI(w, r)
}
