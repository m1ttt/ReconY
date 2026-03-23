package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// mountSPA serves the compiled React SPA from web/dist.
// It falls back to index.html for unknown paths (SPA routing).
func mountSPA(r chi.Router, uiDir string) {
	if uiDir == "" {
		// Try to find web/dist relative to the binary or cwd
		// Priority: $RECONX_UI_DIR > ./web/dist > (binary dir)/web/dist
		if v := os.Getenv("RECONX_UI_DIR"); v != "" {
			uiDir = v
		} else {
			cwd, _ := os.Getwd()
			candidate := filepath.Join(cwd, "web", "dist")
			if _, err := os.Stat(candidate); err == nil {
				uiDir = candidate
			}
		}
	}

	if uiDir == "" {
		// No UI found — API-only mode
		return
	}

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return
	}

	fsys := http.Dir(uiDir)
	fileServer := http.StripPrefix("/", http.FileServer(fsys))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// Check if the file actually exists; if not, serve index.html (SPA fallback)
		path := req.URL.Path
		if path == "/" || path == "" {
			http.ServeFile(w, req, filepath.Join(uiDir, "index.html"))
			return
		}
		fullPath := filepath.Join(uiDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			http.ServeFile(w, req, filepath.Join(uiDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, req)
	})
}
