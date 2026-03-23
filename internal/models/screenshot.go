package models

// Screenshot represents a captured screenshot of a URL.
type Screenshot struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	SubdomainID *string `json:"subdomain_id,omitempty" db:"subdomain_id"`
	URL         string  `json:"url" db:"url"`
	FilePath    string  `json:"file_path" db:"file_path"`
	StatusCode  *int    `json:"status_code,omitempty" db:"status_code"`
	Title       *string `json:"title,omitempty" db:"title"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
