package models

// HistoricalURL represents a URL found via waybackurls or gau.
type HistoricalURL struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	URL         string  `json:"url" db:"url"`
	Source      string  `json:"source" db:"source"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}

// DiscoveredURL represents a URL found via crawling or fuzzing.
type DiscoveredURL struct {
	ID               string  `json:"id" db:"id"`
	WorkspaceID      string  `json:"workspace_id" db:"workspace_id"`
	SubdomainID      *string `json:"subdomain_id,omitempty" db:"subdomain_id"`
	URL              string  `json:"url" db:"url"`
	StatusCode       *int    `json:"status_code,omitempty" db:"status_code"`
	ContentType      *string `json:"content_type,omitempty" db:"content_type"`
	ContentLength    *int    `json:"content_length,omitempty" db:"content_length"`
	RedirectLocation *string `json:"redirect_location,omitempty" db:"redirect_location"`
	Source           string  `json:"source" db:"source"`
	ScanJobID        *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt        string  `json:"created_at" db:"created_at"`
}

// Parameter represents a discovered URL parameter.
type Parameter struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	URL         string  `json:"url" db:"url"`
	Name        string  `json:"name" db:"name"`
	ParamType   string  `json:"param_type" db:"param_type"`
	Source      string  `json:"source" db:"source"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
