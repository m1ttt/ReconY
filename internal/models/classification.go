package models

// SiteType classifies what kind of web application a site is.
type SiteType string

const (
	SiteTypeSPA     SiteType = "spa"
	SiteTypeSSR     SiteType = "ssr"
	SiteTypeHybrid  SiteType = "hybrid"
	SiteTypeClassic SiteType = "classic"
	SiteTypeAPI     SiteType = "api"
	SiteTypeUnknown SiteType = "unknown"
)

// InfraType classifies the hosting infrastructure.
type InfraType string

const (
	InfraTypeBareMetal   InfraType = "bare_metal"
	InfraTypeContainer   InfraType = "container"
	InfraTypeServerless  InfraType = "serverless"
	InfraTypeUnknown     InfraType = "unknown"
)

// SiteClassification stores the fingerprint classification for a URL.
type SiteClassification struct {
	ID          string   `json:"id" db:"id"`
	WorkspaceID string   `json:"workspace_id" db:"workspace_id"`
	SubdomainID *string  `json:"subdomain_id,omitempty" db:"subdomain_id"`
	URL         string   `json:"url" db:"url"`
	SiteType    SiteType `json:"site_type" db:"site_type"`
	InfraType   *string  `json:"infra_type,omitempty" db:"infra_type"`
	WAFDetected *string  `json:"waf_detected,omitempty" db:"waf_detected"`
	CDNDetected *string  `json:"cdn_detected,omitempty" db:"cdn_detected"`
	SSLGrade    *string  `json:"ssl_grade,omitempty" db:"ssl_grade"`
	SSLDetails  *string  `json:"ssl_details,omitempty" db:"ssl_details"`  // JSON
	Evidence    *string  `json:"evidence,omitempty" db:"evidence"`        // JSON
	ScanJobID   *string  `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt   string   `json:"created_at" db:"created_at"`
}
