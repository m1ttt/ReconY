package models

// DNSRecord represents a DNS record for a host.
type DNSRecord struct {
	ID          string  `json:"id" db:"id"`
	WorkspaceID string  `json:"workspace_id" db:"workspace_id"`
	Host        string  `json:"host" db:"host"`
	RecordType  string  `json:"record_type" db:"record_type"`
	Value       string  `json:"value" db:"value"`
	TTL         *int    `json:"ttl,omitempty" db:"ttl"`
	Priority    *int    `json:"priority,omitempty" db:"priority"`
	ScanJobID   *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
}
