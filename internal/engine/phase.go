package engine

import (
	"context"

	"reconx/internal/config"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// PhaseID identifies a recon phase.
type PhaseID int

const (
	PhasePassive     PhaseID = 1
	PhaseSubdomains  PhaseID = 2
	PhasePorts       PhaseID = 3
	PhaseFingerprint PhaseID = 4
	PhaseContent     PhaseID = 5
	PhaseCloud       PhaseID = 6
	PhaseVulns       PhaseID = 7
)

// PhaseName returns a human-readable name for the phase.
func PhaseName(id PhaseID) string {
	switch id {
	case PhasePassive:
		return "Passive Recon"
	case PhaseSubdomains:
		return "Subdomain Enumeration"
	case PhasePorts:
		return "Port Scanning"
	case PhaseFingerprint:
		return "Fingerprinting"
	case PhaseContent:
		return "Content Discovery"
	case PhaseCloud:
		return "Cloud & Secrets"
	case PhaseVulns:
		return "Vulnerability Scanning"
	default:
		return "Unknown"
	}
}

// ToolRunner wraps a single external recon tool.
type ToolRunner interface {
	// Name returns the tool identifier (e.g., "subfinder", "nmap").
	Name() string

	// Phase returns which phase this tool belongs to.
	Phase() PhaseID

	// Check verifies the tool binary is available.
	Check() error

	// Run executes the tool. Results are pushed through the ResultSink.
	// The context may be cancelled for timeout or user cancellation.
	Run(ctx context.Context, input *PhaseInput, sink ResultSink) error
}

// PhaseInput carries data from previous phases into the current one.
type PhaseInput struct {
	Workspace       *models.Workspace
	Subdomains      []models.Subdomain
	Ports           []models.Port
	Technologies    []models.Technology
	Classifications []models.SiteClassification
	DNSRecords      []models.DNSRecord
	WhoisRecords    []models.WhoisRecord
	DiscoveredURLs  []models.DiscoveredURL // URLs from previous scans (for JS analysis, etc.)
	HistoricalURLs  []models.HistoricalURL // URLs from waybackurls/gau
	Config          *config.Config
	ScanJobID       string
	ProxyURL        string                 // proxy URL for external tool env injection
	AuthSessions    []*httpkit.AuthSession // active auth sessions for authenticated crawling
}

// TargetFilter allows scoping tool execution to specific targets.
// Used in interactive recon mode where the user selects specific items.
type TargetFilter struct {
	SubdomainIDs []string `json:"subdomain_ids,omitempty"`
	Hostnames    []string `json:"hostnames,omitempty"`
	PortIDs      []string `json:"port_ids,omitempty"`
	URLIDs       []string `json:"url_ids,omitempty"`
}

// IsEmpty returns true if no filters are set.
func (tf *TargetFilter) IsEmpty() bool {
	if tf == nil {
		return true
	}
	return len(tf.SubdomainIDs) == 0 && len(tf.Hostnames) == 0 && len(tf.PortIDs) == 0 && len(tf.URLIDs) == 0
}

// ResultSink receives normalized results during a tool run.
type ResultSink interface {
	AddSubdomain(ctx context.Context, s *models.Subdomain) error
	AddPort(ctx context.Context, p *models.Port) error
	AddTechnology(ctx context.Context, t *models.Technology) error
	AddVulnerability(ctx context.Context, v *models.Vulnerability) error
	AddDNSRecord(ctx context.Context, r *models.DNSRecord) error
	AddWhoisRecord(ctx context.Context, w *models.WhoisRecord) error
	AddHistoricalURL(ctx context.Context, u *models.HistoricalURL) error
	AddDiscoveredURL(ctx context.Context, u *models.DiscoveredURL) error
	AddParameter(ctx context.Context, p *models.Parameter) error
	AddScreenshot(ctx context.Context, s *models.Screenshot) error
	AddCloudAsset(ctx context.Context, c *models.CloudAsset) error
	AddSecret(ctx context.Context, s *models.Secret) error
	AddSiteClassification(ctx context.Context, c *models.SiteClassification) error
	LogLine(ctx context.Context, stream string, line string)
}
