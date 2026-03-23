package models

// WhoisRecord stores WHOIS and ASN information for a domain.
type WhoisRecord struct {
	ID           string  `json:"id" db:"id"`
	WorkspaceID  string  `json:"workspace_id" db:"workspace_id"`
	Domain       string  `json:"domain" db:"domain"`
	Registrar    *string `json:"registrar,omitempty" db:"registrar"`
	Org          *string `json:"org,omitempty" db:"org"`
	Country      *string `json:"country,omitempty" db:"country"`
	CreationDate *string `json:"creation_date,omitempty" db:"creation_date"`
	ExpiryDate   *string `json:"expiry_date,omitempty" db:"expiry_date"`
	NameServers  *string `json:"name_servers,omitempty" db:"name_servers"` // JSON array
	Raw          *string `json:"raw,omitempty" db:"raw"`
	ASN          *string `json:"asn,omitempty" db:"asn"`
	ASNOrg       *string `json:"asn_org,omitempty" db:"asn_org"`
	ASNCIDR      *string `json:"asn_cidr,omitempty" db:"asn_cidr"` // JSON array
	ScanJobID    *string `json:"scan_job_id,omitempty" db:"scan_job_id"`
	CreatedAt    string  `json:"created_at" db:"created_at"`
}
