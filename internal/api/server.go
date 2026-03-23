package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"reconx/internal/config"
	"reconx/internal/engine"
	"reconx/internal/workflow"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server holds API dependencies.
type Server struct {
	DB        *sql.DB
	Engine    *engine.Engine
	EventBus  *engine.EventBus
	Config    *config.Config
	Workflows *workflow.Registry
}

// NewRouter builds the Chi router with all API routes.
func (s *Server) NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		// Workspaces
		r.Route("/workspaces", func(r chi.Router) {
			r.Get("/", s.listWorkspaces)
			r.Post("/", s.createWorkspace)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.getWorkspace)
				r.Put("/", s.updateWorkspace)
				r.Delete("/", s.deleteWorkspace)
				r.Get("/stats", s.getWorkspaceStats)
				r.Get("/config", s.getWorkspaceConfig)
				r.Put("/config", s.setWorkspaceConfig)

				// Scans
				r.Post("/scans", s.startScan)
				r.Get("/scans", s.listScans)
				r.Get("/scans/{jobId}", s.getScan)
				r.Post("/scans/{jobId}/cancel", s.cancelScan)
				r.Get("/scans/{jobId}/logs", s.getScanLogs)

				// Results
				r.Get("/subdomains", s.listSubdomains)
				r.Get("/ports", s.listPorts)
				r.Get("/technologies", s.listTechnologies)
				r.Get("/vulnerabilities", s.listVulnerabilities)
				r.Get("/dns", s.listDNSRecords)
				r.Get("/whois", s.listWhoisRecords)
				r.Get("/historical-urls", s.listHistoricalURLs)
				r.Get("/urls", s.listDiscoveredURLs)
				r.Get("/parameters", s.listParameters)
				r.Get("/screenshots", s.listScreenshots)
				r.Get("/screenshots/{screenshotId}/image", s.serveScreenshot)
				r.Get("/cloud-assets", s.listCloudAssets)
				r.Get("/secrets", s.listSecrets)
				r.Get("/classifications", s.listClassifications)
				r.Get("/export", s.exportWorkspace)

				// Auth Credentials
				r.Route("/auth", func(r chi.Router) {
					r.Get("/", s.listAuthCredentials)
					r.Post("/", s.createAuthCredential)
					r.Route("/{credId}", func(r chi.Router) {
						r.Put("/", s.updateAuthCredential)
						r.Delete("/", s.deleteAuthCredential)
						r.Post("/test", s.testAuthCredential)
					})
				})
			})
		})

		// Workflows
		r.Route("/workflows", func(r chi.Router) {
			r.Get("/", s.listWorkflows)
			r.Post("/", s.createWorkflow)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.getWorkflow)
				r.Put("/", s.updateWorkflow)
				r.Delete("/", s.deleteWorkflow)
				r.Post("/duplicate", s.duplicateWorkflow)
			})
		})

		// Config
		r.Get("/config", s.getConfig)
		r.Put("/config", s.updateConfig)

		// System
		r.Get("/tools/check", s.checkTools)
		r.Get("/tools/registry", s.getToolRegistry)
	})

	return r
}

// JSON helpers

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
