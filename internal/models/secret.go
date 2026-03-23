package models

// Secret represents a discovered secret (API key, token, etc.).
type Secret struct {
	ID          string   `json:"id" db:"id"`
	WorkspaceID string   `json:"workspace_id" db:"workspace_id"`
	SourceURL   string   `json:"source_url" db:"source_url"`
	SecretType  string   `json:"secret_type" db:"secret_type"`
	Value       string   `json:"value" db:"value"`
	Context     *string  `json:"context,omitempty" db:"context"`
	Source      string   `json:"source" db:"source"`
	Severity    Severity `json:"severity" db:"severity"`
	ScanJobID   *string  `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string   `json:"created_at" db:"created_at"`
}
