package models

// Workspace represents a recon engagement for a target domain.
type Workspace struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Domain      string `json:"domain" db:"domain"`
	Description string `json:"description" db:"description"`
	ConfigJSON  string `json:"config_json" db:"config_json"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`
}

// WorkspaceStats contains aggregated counts for a workspace.
type WorkspaceStats struct {
	Subdomains      int `json:"subdomains"`
	AliveSubdomains int `json:"alive_subdomains"`
	OpenPorts       int `json:"open_ports"`
	Technologies    int `json:"technologies"`
	Vulnerabilities int `json:"vulnerabilities"`
	Secrets         int `json:"secrets"`
	Screenshots     int `json:"screenshots"`
	CloudAssets     int `json:"cloud_assets"`
}
