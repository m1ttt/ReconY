package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"reconx/internal/models"
	"github.com/go-chi/chi/v5"
)

type workspaceRequest struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
}

type workspaceResponse struct {
	models.Workspace
	Stats *models.WorkspaceStats `json:"stats,omitempty"`
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT w.id, w.name, w.domain, w.description, w.config_json, w.created_at, w.updated_at,
			(SELECT COUNT(*) FROM subdomains WHERE workspace_id = w.id) as sub_count,
			(SELECT COUNT(*) FROM subdomains WHERE workspace_id = w.id AND is_alive = 1) as alive_count,
			(SELECT COUNT(*) FROM ports WHERE workspace_id = w.id AND state = 'open') as port_count,
			(SELECT COUNT(*) FROM technologies WHERE workspace_id = w.id) as tech_count,
			(SELECT COUNT(*) FROM vulnerabilities WHERE workspace_id = w.id) as vuln_count,
			(SELECT COUNT(*) FROM secrets WHERE workspace_id = w.id) as secret_count,
			(SELECT COUNT(*) FROM screenshots WHERE workspace_id = w.id) as screenshot_count,
			(SELECT COUNT(*) FROM cloud_assets WHERE workspace_id = w.id) as cloud_count
		FROM workspaces w ORDER BY w.created_at DESC
	`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var results []workspaceResponse
	for rows.Next() {
		var ws models.Workspace
		var stats models.WorkspaceStats
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Domain, &ws.Description, &ws.ConfigJSON, &ws.CreatedAt, &ws.UpdatedAt,
			&stats.Subdomains, &stats.AliveSubdomains, &stats.OpenPorts, &stats.Technologies,
			&stats.Vulnerabilities, &stats.Secrets, &stats.Screenshots, &stats.CloudAssets); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		results = append(results, workspaceResponse{Workspace: ws, Stats: &stats})
	}

	if results == nil {
		results = []workspaceResponse{}
	}
	writeJSON(w, 200, results)
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req workspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Domain == "" {
		writeError(w, 400, "domain is required")
		return
	}
	if req.Name == "" {
		req.Name = req.Domain
	}

	id := models.NewID()
	now := models.Now()
	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO workspaces (id, name, domain, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, req.Name, req.Domain, req.Description, now, now)
	if err != nil {
		writeError(w, 409, fmt.Sprintf("workspace creation failed: %v", err))
		return
	}

	ws := models.Workspace{
		ID: id, Name: req.Name, Domain: req.Domain,
		Description: req.Description, ConfigJSON: "{}",
		CreatedAt: now, UpdatedAt: now,
	}
	writeJSON(w, 201, ws)
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var ws models.Workspace
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT id, name, domain, description, config_json, created_at, updated_at
		FROM workspaces WHERE id = ?
	`, id).Scan(&ws.ID, &ws.Name, &ws.Domain, &ws.Description, &ws.ConfigJSON, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		writeError(w, 404, "workspace not found")
		return
	}

	writeJSON(w, 200, ws)
}

func (s *Server) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req workspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	now := models.Now()
	result, err := s.DB.ExecContext(r.Context(), `
		UPDATE workspaces SET name = COALESCE(NULLIF(?, ''), name),
			description = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.Description, now, id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "workspace not found")
		return
	}

	s.getWorkspace(w, r)
}

func (s *Server) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := s.DB.ExecContext(r.Context(), "DELETE FROM workspaces WHERE id = ?", id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "workspace not found")
		return
	}

	w.WriteHeader(204)
}

func (s *Server) getWorkspaceStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var stats models.WorkspaceStats
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM subdomains WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM subdomains WHERE workspace_id = ? AND is_alive = 1),
			(SELECT COUNT(*) FROM ports WHERE workspace_id = ? AND state = 'open'),
			(SELECT COUNT(*) FROM technologies WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM vulnerabilities WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM secrets WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM screenshots WHERE workspace_id = ?),
			(SELECT COUNT(*) FROM cloud_assets WHERE workspace_id = ?)
	`, id, id, id, id, id, id, id, id).Scan(
		&stats.Subdomains, &stats.AliveSubdomains, &stats.OpenPorts, &stats.Technologies,
		&stats.Vulnerabilities, &stats.Secrets, &stats.Screenshots, &stats.CloudAssets,
	)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, 200, stats)
}

func (s *Server) getWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var configJSON string
	err := s.DB.QueryRowContext(r.Context(), "SELECT config_json FROM workspaces WHERE id = ?", id).Scan(&configJSON)
	if err != nil {
		writeError(w, 404, "workspace not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte(configJSON))
}

func (s *Server) setWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var configMap map[string]any
	if err := decodeJSON(r, &configMap); err != nil {
		writeError(w, 400, "invalid JSON config")
		return
	}

	// Re-encode to ensure valid JSON
	encoded, _ := json.Marshal(configMap)

	now := models.Now()
	result, err := s.DB.ExecContext(r.Context(), `
		UPDATE workspaces SET config_json = ?, updated_at = ? WHERE id = ?
	`, string(encoded), now, id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "workspace not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(encoded)
}

func (s *Server) exportWorkspace(w http.ResponseWriter, r *http.Request) {
	// TODO: implement full export (JSON/CSV)
	writeError(w, 501, "export not yet implemented")
}
