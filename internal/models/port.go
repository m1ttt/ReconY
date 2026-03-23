package models

// Port represents an open/filtered port on a host.
type Port struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	SubdomainID *string `json:"subdomain_id,omitempty" db:"subdomain_id"`
	IPAddress   string  `json:"ip_address" db:"ip_address"`
	Port        int     `json:"port" db:"port"`
	Protocol    string  `json:"protocol" db:"protocol"`
	State       string  `json:"state" db:"state"`
	Service     *string `json:"service,omitempty" db:"service"`
	Version     *string `json:"version,omitempty" db:"version"`
	Banner      *string `json:"banner,omitempty" db:"banner"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
