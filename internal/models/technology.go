package models

// Technology represents a detected technology on a URL.
type Technology struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	SubdomainID *string `json:"subdomain_id,omitempty" db:"subdomain_id"`
	URL         string  `json:"url" db:"url"`
	Name        string  `json:"name" db:"name"`
	Version     *string `json:"version,omitempty" db:"version"`
	Category    *string `json:"category,omitempty" db:"category"`
	Confidence  int     `json:"confidence" db:"confidence"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
