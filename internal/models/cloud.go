package models

// CloudAsset represents a discovered cloud resource (S3 bucket, Azure blob, etc.).
type CloudAsset struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	Provider    string  `json:"provider" db:"provider"`
	AssetType   string  `json:"asset_type" db:"asset_type"`
	Name        string  `json:"name" db:"name"`
	URL         *string `json:"url,omitempty" db:"url"`
	IsPublic    bool    `json:"is_public" db:"is_public"`
	Permissions *string `json:"permissions,omitempty" db:"permissions"` // JSON
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
