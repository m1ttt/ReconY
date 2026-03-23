package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"reconx/internal/engine"
	"reconx/internal/models"
	"github.com/go-chi/chi/v5"
)

// activeScanCancels tracks cancel functions for running scans.
var (
	activeScanCancels = make(map[string]context.CancelFunc)
	scanMu            sync.Mutex
)

type startScanRequest struct {
	Workflow string               `json:"workflow,omitempty"`
	Phases   []int                `json:"phases,omitempty"`
	Tool     string               `json:"tool,omitempty"`
	Targets  *engine.TargetFilter `json:"targets,omitempty"`
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")

	// Verify workspace exists
	var exists int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM workspaces WHERE id = ?", wsID).Scan(&exists); err != nil || exists == 0 {
		writeError(w, 404, "workspace not found")
		return
	}

	var req startScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	scanID := models.NewID()

	scanMu.Lock()
	activeScanCancels[scanID] = cancel
	scanMu.Unlock()

	// Run scan in background
	go func() {
		defer func() {
			scanMu.Lock()
			delete(activeScanCancels, scanID)
			scanMu.Unlock()
			cancel()
		}()

		var err error
		switch {
		case req.Tool != "" && !req.Targets.IsEmpty():
			err = s.Engine.RunToolWithTargets(ctx, wsID, req.Tool, req.Targets)
		case req.Tool != "":
			err = s.Engine.RunTool(ctx, wsID, req.Tool)
		case len(req.Phases) > 0:
			err = s.Engine.RunPhases(ctx, wsID, req.Phases)
		case req.Workflow != "":
			wf, ok := s.Workflows.Get(req.Workflow)
			if !ok {
				return
			}
			err = s.Engine.RunWorkflow(ctx, wsID, wf)
		default:
			wf, ok := s.Workflows.Get(s.Config.General.DefaultWorkflow)
			if !ok {
				return
			}
			err = s.Engine.RunWorkflow(ctx, wsID, wf)
		}
		_ = err // errors already logged per-tool in engine
	}()

	writeJSON(w, 202, map[string]string{
		"message":  "scan started",
		"scan_id":  scanID,
		"status":   "running",
	})
}

func (s *Server) listScans(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")

	phase := r.URL.Query().Get("phase")
	status := r.URL.Query().Get("status")

	query := `SELECT id, workspace_id, phase, tool_name, status, started_at, finished_at,
		result_count, error_message, created_at FROM scan_jobs WHERE workspace_id = ?`
	args := []any{wsID}

	if phase != "" {
		query += " AND phase = ?"
		args = append(args, phase)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var jobs []models.ScanJob
	for rows.Next() {
		var j models.ScanJob
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.Phase, &j.ToolName, &j.Status,
			&j.StartedAt, &j.FinishedAt, &j.ResultCount, &j.ErrorMessage, &j.CreatedAt); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []models.ScanJob{}
	}

	writeJSON(w, 200, jobs)
}

func (s *Server) getScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	var j models.ScanJob
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT id, workspace_id, phase, tool_name, status, started_at, finished_at,
			result_count, error_message, created_at
		FROM scan_jobs WHERE id = ?
	`, jobID).Scan(&j.ID, &j.WorkspaceID, &j.Phase, &j.ToolName, &j.Status,
		&j.StartedAt, &j.FinishedAt, &j.ResultCount, &j.ErrorMessage, &j.CreatedAt)
	if err != nil {
		writeError(w, 404, "scan job not found")
		return
	}

	writeJSON(w, 200, j)
}

func (s *Server) cancelScan(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	scanMu.Lock()
	cancel, ok := activeScanCancels[jobID]
	scanMu.Unlock()

	if !ok {
		writeError(w, 404, "no active scan with this ID")
		return
	}

	cancel()
	writeJSON(w, 200, map[string]string{"message": fmt.Sprintf("scan %s cancelled", jobID)})
}

func (s *Server) getScanLogs(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT stream, line, timestamp FROM tool_logs
		WHERE scan_job_id = ? ORDER BY id
	`, jobID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type logEntry struct {
		Stream    string `json:"stream"`
		Line      string `json:"line"`
		Timestamp string `json:"timestamp"`
	}

	var logs []logEntry
	for rows.Next() {
		var l logEntry
		if err := rows.Scan(&l.Stream, &l.Line, &l.Timestamp); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []logEntry{}
	}

	writeJSON(w, 200, logs)
}
