package models

// Subdomain represents a discovered subdomain.
type Subdomain struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	Hostname    string  `json:"hostname" db:"hostname"`
	IPAddresses *string `json:"ip_addresses,omitempty" db:"ip_addresses"` // JSON array
	IsAlive     bool    `json:"is_alive" db:"is_alive"`
	Source      string  `json:"source" db:"source"`
	FirstSeen   string  `json:"first_seen" db:"first_seen"`
	LastSeen    string  `json:"last_seen" db:"last_seen"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
}
