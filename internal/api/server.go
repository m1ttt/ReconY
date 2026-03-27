package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

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

		// AI
		r.Post("/ai/ask", s.askAI)

		// IP Info
		r.Get("/ip-info", s.getIPInfo)

		// Mullvad status & rotation
		r.Get("/mullvad-status", s.getMullvadStatus)
		r.Post("/mullvad-rotate", s.rotateMullvad)
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

// ── IP / Mullvad ─────────────────────────────────────────────────────────────

// IPInfoResponse is sent to the frontend header.
type IPInfoResponse struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city,omitempty"`
	IsProxy     bool   `json:"is_proxy"`
	IsTor       bool   `json:"is_tor"`
}

// mullvadStatusJSON matches the output of `mullvad status --json`.
type mullvadStatusJSON struct {
	State   string `json:"state"`
	Details struct {
		Location *struct {
			IPV4        string `json:"ipv4"`
			Country     string `json:"country"`
			City        string `json:"city"`
			CountryCode string `json:"country_code"`
			Hostname    string `json:"hostname"`
		} `json:"location"`
	} `json:"details"`
}

// parseMullvadJSON runs `mullvad status --json` and decodes it.
func parseMullvadJSON() (*mullvadStatusJSON, error) {
	out, err := exec.Command("mullvad", "status", "--json").Output()
	if err != nil {
		return nil, err
	}
	var s mullvadStatusJSON
	return &s, json.Unmarshal(out, &s)
}

// fetchPublicIP calls http://soporteweb.com which returns the raw public IP
// as plain text — no rate limits, no API key needed.
func fetchPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://soporteweb.com/")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// getIPInfo returns the server's real public IP + geo info.
// When Mullvad is connected it reads everything from the CLI JSON (zero HTTP).
// Otherwise fetches the IP from soporteweb.com.
func (s *Server) getIPInfo(w http.ResponseWriter, r *http.Request) {
	// Mullvad connected → use CLI data directly, no HTTP call needed
	if s.Config.Proxy.MullvadCLI {
		if ms, err := parseMullvadJSON(); err == nil &&
			ms.State == "connected" &&
			ms.Details.Location != nil {
			loc := ms.Details.Location
			writeJSON(w, http.StatusOK, IPInfoResponse{
				IP:          loc.IPV4,
				Country:     loc.Country,
				CountryCode: loc.CountryCode,
				City:        loc.City,
				IsProxy:     true,
			})
			return
		}
	}

	// Fallback: plain-text IP from soporteweb.com
	ip := fetchPublicIP()
	if ip == "" {
		ip = "Unknown"
	}
	writeJSON(w, http.StatusOK, IPInfoResponse{IP: ip, Country: "—"})
}

// MullvadStatusResponse is the payload for GET /api/v1/mullvad-status.
type MullvadStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Status    string `json:"status"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`
	IP        string `json:"ip,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

// getMullvadStatus returns the current Mullvad VPN state parsed from the CLI.
func (s *Server) getMullvadStatus(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Proxy.MullvadCLI {
		writeJSON(w, http.StatusOK, MullvadStatusResponse{Enabled: false})
		return
	}

	ms, err := parseMullvadJSON()
	if err != nil {
		writeJSON(w, http.StatusOK, MullvadStatusResponse{
			Enabled: true, Connected: false, Status: "unavailable",
		})
		return
	}

	resp := MullvadStatusResponse{
		Enabled:   true,
		Connected: ms.State == "connected",
		Status:    ms.State,
	}
	if ms.Details.Location != nil {
		resp.Country = ms.Details.Location.Country
		resp.City = ms.Details.Location.City
		resp.IP = ms.Details.Location.IPV4
		resp.Hostname = ms.Details.Location.Hostname
	}

	writeJSON(w, http.StatusOK, resp)
}

// rotateMullvad switches the Mullvad relay to the requested location.
// POST /api/v1/mullvad-rotate  body: {"location": "de"}
func (s *Server) rotateMullvad(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Proxy.MullvadCLI {
		writeError(w, http.StatusBadRequest, "mullvad_cli not enabled")
		return
	}

	var body struct {
		Location string `json:"location"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Location == "" {
		writeError(w, http.StatusBadRequest, "location required")
		return
	}

	// Set relay location
	if out, err := exec.Command("mullvad", "relay", "set", "location", body.Location).CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}

	// Reconnect (--wait blocks until tunnel established)
	if out, err := exec.Command("mullvad", "reconnect", "--wait").CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}

	// Return updated status
	s.getMullvadStatus(w, r)
}
