package api

import (
	"encoding/json"
	"net/http"

	"reconx/internal/models"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	// Builtin workflows from registry
	builtinList := s.Workflows.List()

	// Custom workflows from DB
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, workspace_id, name, description, config, is_builtin, created_at
		FROM workflows ORDER BY created_at DESC
	`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type workflowEntry struct {
		ID          string  `json:"id,omitempty"`
		WorkspaceID *string `json:"workspace_id,omitempty"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		IsBuiltin   bool    `json:"is_builtin"`
		PhaseIDs    []int   `json:"phase_ids,omitempty"`
		Config      string  `json:"config,omitempty"`
		CreatedAt   string  `json:"created_at,omitempty"`
	}

	var results []workflowEntry

	// Add builtins first
	for _, wf := range builtinList {
		results = append(results, workflowEntry{
			Name:        wf.Name,
			Description: wf.Description,
			IsBuiltin:   true,
			PhaseIDs:    wf.PhaseIDs,
		})
	}

	// Add DB custom workflows
	for rows.Next() {
		var entry workflowEntry
		var wsID *string
		var isBuiltin int
		if err := rows.Scan(&entry.ID, &wsID, &entry.Name, &entry.Description, &entry.Config, &isBuiltin, &entry.CreatedAt); err != nil {
			continue
		}
		entry.WorkspaceID = wsID
		entry.IsBuiltin = isBuiltin == 1
		results = append(results, entry)
	}

	if results == nil {
		results = []workflowEntry{}
	}
	writeJSON(w, 200, results)
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		WorkspaceID *string `json:"workspace_id"`
		Config      string  `json:"config"` // YAML string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" || req.Config == "" {
		writeError(w, 400, "name and config are required")
		return
	}

	id := models.NewID()
	now := models.Now()
	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO workflows (id, workspace_id, name, description, config, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, id, req.WorkspaceID, req.Name, req.Description, req.Config, now, now)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, 201, map[string]string{"id": id, "name": req.Name})
}

func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check builtins first
	for _, wf := range s.Workflows.List() {
		if wf.Name == id {
			// Return the full YAML config for builtins
			wfObj, ok := s.Workflows.Get(id)
			if ok {
				yamlData, _ := wfObj.Marshal()
				writeJSON(w, 200, map[string]any{
					"name":        wf.Name,
					"description": wf.Description,
					"is_builtin":  true,
					"phase_ids":   wf.PhaseIDs,
					"config":      string(yamlData),
				})
				return
			}
		}
	}

	// Check DB
	var name, description, config, createdAt string
	var wsID *string
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT name, description, config, workspace_id, created_at
		FROM workflows WHERE id = ?
	`, id).Scan(&name, &description, &config, &wsID, &createdAt)
	if err != nil {
		writeError(w, 404, "workflow not found")
		return
	}

	writeJSON(w, 200, map[string]any{
		"id":           id,
		"name":         name,
		"description":  description,
		"workspace_id": wsID,
		"is_builtin":   false,
		"config":       config,
		"created_at":   createdAt,
	})
}

func (s *Server) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Config      string `json:"config"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	now := models.Now()
	result, err := s.DB.ExecContext(r.Context(), `
		UPDATE workflows SET
			name = COALESCE(NULLIF(?, ''), name),
			description = COALESCE(NULLIF(?, ''), description),
			config = COALESCE(NULLIF(?, ''), config),
			updated_at = ?
		WHERE id = ? AND is_builtin = 0
	`, req.Name, req.Description, req.Config, now, id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "workflow not found or is builtin (cannot modify)")
		return
	}

	s.getWorkflow(w, r)
}

func (s *Server) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := s.DB.ExecContext(r.Context(), "DELETE FROM workflows WHERE id = ? AND is_builtin = 0", id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "workflow not found or is builtin")
		return
	}

	w.WriteHeader(204)
}

func (s *Server) duplicateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if it's a builtin name
	wfObj, ok := s.Workflows.Get(id)
	var name, description, config string

	if ok {
		name = wfObj.Name + "-copy"
		description = wfObj.Description
		yamlData, _ := wfObj.Marshal()
		config = string(yamlData)
	} else {
		// Try DB
		err := s.DB.QueryRowContext(r.Context(), `
			SELECT name, description, config FROM workflows WHERE id = ?
		`, id).Scan(&name, &description, &config)
		if err != nil {
			writeError(w, 404, "workflow not found")
			return
		}
		name = name + "-copy"
	}

	newID := models.NewID()
	now := models.Now()
	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO workflows (id, name, description, config, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
	`, newID, name, description, config, now, now)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, 201, map[string]string{"id": newID, "name": name})
}

// Unused import suppressor
var _ = json.Marshal
