package api

import (
	"context"
	"net/http"
	"strings"

	"reconx/internal/httpkit"
	"reconx/internal/models"

	"github.com/go-chi/chi/v5"
)

type authCredRequest struct {
	Name        string          `json:"name"`
	AuthType    models.AuthType `json:"auth_type"`
	Username    *string         `json:"username,omitempty"`
	Password    *string         `json:"password,omitempty"`
	LoginURL    *string         `json:"login_url,omitempty"`
	LoginBody   *string         `json:"login_body,omitempty"`
	Token       *string         `json:"token,omitempty"`
	HeaderName  *string         `json:"header_name,omitempty"`
	HeaderValue *string         `json:"header_value,omitempty"`
	IsActive    *bool           `json:"is_active,omitempty"`
}

func (s *Server) listAuthCredentials(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, workspace_id, name, auth_type, username, login_url, login_body,
			header_name, is_active, created_at, updated_at
		FROM auth_credentials WHERE workspace_id = ? ORDER BY created_at DESC
	`, wsID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type safeCredential struct {
		ID          string          `json:"id"`
		WorkspaceID string          `json:"workspace_id"`
		Name        string          `json:"name"`
		AuthType    models.AuthType `json:"auth_type"`
		Username    *string         `json:"username,omitempty"`
		LoginURL    *string         `json:"login_url,omitempty"`
		LoginBody   *string         `json:"login_body,omitempty"`
		HeaderName  *string         `json:"header_name,omitempty"`
		IsActive    bool            `json:"is_active"`
		CreatedAt   string          `json:"created_at"`
		UpdatedAt   string          `json:"updated_at"`
	}

	var results []safeCredential
	for rows.Next() {
		var c safeCredential
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.AuthType,
			&c.Username, &c.LoginURL, &c.LoginBody,
			&c.HeaderName, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		results = append(results, c)
	}
	if results == nil {
		results = []safeCredential{}
	}
	writeJSON(w, 200, results)
}

func (s *Server) createAuthCredential(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")

	var req authCredRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, 400, "name is required")
		return
	}
	if req.AuthType == "" {
		req.AuthType = models.AuthTypeNone
	}

	validTypes := map[models.AuthType]bool{
		models.AuthTypeNone: true, models.AuthTypeBasic: true, models.AuthTypeForm: true,
		models.AuthTypeCookie: true, models.AuthTypeBearer: true, models.AuthTypeHeader: true,
	}
	if !validTypes[req.AuthType] {
		writeError(w, 400, "invalid auth_type")
		return
	}

	id := models.NewID()
	now := models.Now()

	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO auth_credentials (id, workspace_id, name, auth_type, username, password,
			login_url, login_body, token, header_name, header_value, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
	`, id, wsID, req.Name, req.AuthType, req.Username, req.Password,
		req.LoginURL, req.LoginBody, req.Token, req.HeaderName, req.HeaderValue,
		now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, 409, "credential with this name already exists")
			return
		}
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, 201, map[string]string{"id": id, "name": req.Name, "status": "created"})
}

func (s *Server) updateAuthCredential(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credId")

	var req authCredRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	now := models.Now()
	result, err := s.DB.ExecContext(r.Context(), `
		UPDATE auth_credentials SET
			name = COALESCE(NULLIF(?, ''), name),
			auth_type = COALESCE(NULLIF(?, ''), auth_type),
			username = ?, password = ?, login_url = ?, login_body = ?,
			token = ?, header_name = ?, header_value = ?,
			is_active = COALESCE(?, is_active),
			updated_at = ?
		WHERE id = ?
	`, req.Name, string(req.AuthType),
		req.Username, req.Password, req.LoginURL, req.LoginBody,
		req.Token, req.HeaderName, req.HeaderValue,
		req.IsActive, now, credID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "credential not found")
		return
	}

	writeJSON(w, 200, map[string]string{"id": credID, "status": "updated"})
}

func (s *Server) deleteAuthCredential(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credId")

	result, err := s.DB.ExecContext(r.Context(), "DELETE FROM auth_credentials WHERE id = ?", credID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, 404, "credential not found")
		return
	}

	w.WriteHeader(204)
}

func (s *Server) testAuthCredential(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credId")

	var cred models.AuthCredential
	err := s.DB.QueryRowContext(r.Context(), `
		SELECT id, workspace_id, name, auth_type, username, password,
			login_url, login_body, token, header_name, header_value,
			is_active, created_at, updated_at
		FROM auth_credentials WHERE id = ?
	`, credID).Scan(&cred.ID, &cred.WorkspaceID, &cred.Name, &cred.AuthType,
		&cred.Username, &cred.Password, &cred.LoginURL, &cred.LoginBody,
		&cred.Token, &cred.HeaderName, &cred.HeaderValue,
		&cred.IsActive, &cred.CreatedAt, &cred.UpdatedAt)
	if err != nil {
		writeError(w, 404, "credential not found")
		return
	}

	sess := httpkit.NewAuthSession(&cred)
	ctx, cancel := context.WithTimeout(r.Context(), 15*1e9) // 15 seconds
	defer cancel()

	client := httpkit.NewClient(s.Config)
	if err := sess.Login(ctx, client); err != nil {
		writeJSON(w, 200, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	result := map[string]any{
		"success":   true,
		"auth_type": cred.AuthType,
	}

	if cookieHeader := sess.CookieHeader(); cookieHeader != "" {
		result["cookies_count"] = len(strings.Split(cookieHeader, ";"))
	}

	writeJSON(w, 200, result)
}
