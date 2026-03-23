package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// pagination parses page/per_page query params.
type pagination struct {
	Page    int
	PerPage int
	Offset  int
}

func parsePagination(r *http.Request) pagination {
	p := pagination{Page: 1, PerPage: 50}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			p.PerPage = n
		}
	}
	p.Offset = (p.Page - 1) * p.PerPage
	return p
}

// Generic paginated list helper for result tables.
func (s *Server) paginatedList(w http.ResponseWriter, r *http.Request, table string, columns []string, extraWhere string, extraArgs []any) {
	wsID := chi.URLParam(r, "id")
	pg := parsePagination(r)
	search := r.URL.Query().Get("search")
	sortCol := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	scanJobID := r.URL.Query().Get("scan_job_id")

	// Build query
	cols := strings.Join(columns, ", ")
	where := "workspace_id = ?"
	args := []any{wsID}

	// Filter by scan_job_id (used by interactive recon to show only current step's results)
	if scanJobID != "" {
		where += " AND scan_job_id = ?"
		args = append(args, scanJobID)
	}

	if extraWhere != "" {
		where += " AND " + extraWhere
		args = append(args, extraArgs...)
	}

	if search != "" {
		// Search across text columns (skip numeric/id columns)
		skipCols := map[string]bool{
			"id": true, "workspace_id": true, "scan_job_id": true, "subdomain_id": true,
			"port": true, "ttl": true, "priority": true, "confidence": true,
			"status_code": true, "content_length": true, "is_alive": true, "is_public": true,
			"result_count": true,
		}
		searchClauses := []string{}
		for _, col := range columns {
			if skipCols[col] {
				continue
			}
			searchClauses = append(searchClauses, fmt.Sprintf("CAST(%s AS TEXT) LIKE ?", col))
			args = append(args, "%"+search+"%")
		}
		if len(searchClauses) > 0 {
			where += " AND (" + strings.Join(searchClauses, " OR ") + ")"
		}
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where)
	s.DB.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)

	// Validate sort column
	validSort := false
	for _, col := range columns {
		if col == sortCol {
			validSort = true
			break
		}
	}
	// Default ordering — use first available timestamp column
	defaultOrder := "created_at DESC"
	for _, col := range columns {
		if col == "first_seen" {
			defaultOrder = "first_seen DESC"
			break
		}
	}
	orderClause := defaultOrder
	if validSort {
		dir := "ASC"
		if strings.ToUpper(sortOrder) == "DESC" {
			dir = "DESC"
		}
		orderClause = fmt.Sprintf("%s %s", sortCol, dir)
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT ? OFFSET ?",
		cols, table, where, orderClause)
	args = append(args, pg.PerPage, pg.Offset)

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	// Scan into generic maps
	colNames := columns
	var results []map[string]any
	for rows.Next() {
		vals := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		row := make(map[string]any)
		for i, col := range colNames {
			v := vals[i]
			// Convert []byte to string
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		results = append(results, row)
	}
	if results == nil {
		results = []map[string]any{}
	}

	writeJSON(w, 200, map[string]any{
		"data":     results,
		"total":    total,
		"page":     pg.Page,
		"per_page": pg.PerPage,
	})
}

func (s *Server) listSubdomains(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if alive := r.URL.Query().Get("alive"); alive == "true" {
		extra = "is_alive = 1"
	}
	if source := r.URL.Query().Get("source"); source != "" {
		if extra != "" {
			extra += " AND "
		}
		extra += "source = ?"
		args = append(args, source)
	}
	s.paginatedList(w, r, "subdomains",
		[]string{"id", "workspace_id", "hostname", "ip_addresses", "is_alive", "source", "first_seen", "last_seen"},
		extra, args)
}

func (s *Server) listPorts(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if state := r.URL.Query().Get("state"); state != "" {
		extra = "state = ?"
		args = append(args, state)
	}
	if service := r.URL.Query().Get("service"); service != "" {
		if extra != "" {
			extra += " AND "
		}
		extra += "service = ?"
		args = append(args, service)
	}
	s.paginatedList(w, r, "ports",
		[]string{"id", "workspace_id", "subdomain_id", "ip_address", "port", "protocol", "state", "service", "version", "banner", "created_at"},
		extra, args)
}

func (s *Server) listTechnologies(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if cat := r.URL.Query().Get("category"); cat != "" {
		extra = "category = ?"
		args = append(args, cat)
	}
	s.paginatedList(w, r, "technologies",
		[]string{"id", "workspace_id", "subdomain_id", "url", "name", "version", "category", "confidence", "created_at"},
		extra, args)
}

func (s *Server) listVulnerabilities(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if sev := r.URL.Query().Get("severity"); sev != "" {
		sevs := strings.Split(sev, ",")
		placeholders := make([]string, len(sevs))
		for i, s := range sevs {
			placeholders[i] = "?"
			args = append(args, strings.TrimSpace(s))
		}
		extra = fmt.Sprintf("severity IN (%s)", strings.Join(placeholders, ","))
	}
	s.paginatedList(w, r, "vulnerabilities",
		[]string{"id", "workspace_id", "subdomain_id", "template_id", "name", "severity", "url", "matched_at", "description", "reference", "curl_command", "created_at"},
		extra, args)
}

func (s *Server) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if rt := r.URL.Query().Get("type"); rt != "" {
		extra = "record_type = ?"
		args = append(args, rt)
	}
	s.paginatedList(w, r, "dns_records",
		[]string{"id", "workspace_id", "host", "record_type", "value", "ttl", "priority", "created_at"},
		extra, args)
}

func (s *Server) listWhoisRecords(w http.ResponseWriter, r *http.Request) {
	s.paginatedList(w, r, "whois_records",
		[]string{"id", "workspace_id", "domain", "registrar", "org", "country", "creation_date", "expiry_date", "name_servers", "asn", "asn_org", "asn_cidr", "created_at"},
		"", nil)
}

func (s *Server) listHistoricalURLs(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if source := r.URL.Query().Get("source"); source != "" {
		extra = "source = ?"
		args = append(args, source)
	}
	s.paginatedList(w, r, "historical_urls",
		[]string{"id", "workspace_id", "url", "source", "created_at"},
		extra, args)
}

func (s *Server) listDiscoveredURLs(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if source := r.URL.Query().Get("source"); source != "" {
		extra = "source = ?"
		args = append(args, source)
	}
	if code := r.URL.Query().Get("status_code"); code != "" {
		if extra != "" {
			extra += " AND "
		}
		extra += "status_code = ?"
		args = append(args, code)
	}
	s.paginatedList(w, r, "discovered_urls",
		[]string{"id", "workspace_id", "subdomain_id", "url", "status_code", "content_type", "content_length", "redirect_location", "source", "created_at"},
		extra, args)
}

func (s *Server) listParameters(w http.ResponseWriter, r *http.Request) {
	s.paginatedList(w, r, "parameters",
		[]string{"id", "workspace_id", "url", "name", "param_type", "source", "created_at"},
		"", nil)
}

func (s *Server) listScreenshots(w http.ResponseWriter, r *http.Request) {
	s.paginatedList(w, r, "screenshots",
		[]string{"id", "workspace_id", "subdomain_id", "url", "file_path", "status_code", "title", "created_at"},
		"", nil)
}

func (s *Server) serveScreenshot(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")
	screenshotID := chi.URLParam(r, "screenshotId")

	var filePath string
	err := s.DB.QueryRowContext(r.Context(),
		"SELECT file_path FROM screenshots WHERE id = ? AND workspace_id = ?",
		screenshotID, wsID).Scan(&filePath)
	if err != nil {
		writeError(w, 404, "screenshot not found")
		return
	}

	http.ServeFile(w, r, filePath)
}

func (s *Server) listCloudAssets(w http.ResponseWriter, r *http.Request) {
	extra := ""
	var args []any
	if provider := r.URL.Query().Get("provider"); provider != "" {
		extra = "provider = ?"
		args = append(args, provider)
	}
	s.paginatedList(w, r, "cloud_assets",
		[]string{"id", "workspace_id", "provider", "asset_type", "name", "url", "is_public", "permissions", "created_at"},
		extra, args)
}

func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	s.paginatedList(w, r, "secrets",
		[]string{"id", "workspace_id", "source_url", "secret_type", "value", "context", "source", "severity", "created_at"},
		"", nil)
}

func (s *Server) listClassifications(w http.ResponseWriter, r *http.Request) {
	s.paginatedList(w, r, "site_classifications",
		[]string{"id", "workspace_id", "subdomain_id", "url", "site_type", "infra_type", "waf_detected", "cdn_detected", "ssl_grade", "evidence", "created_at"},
		"", nil)
}
