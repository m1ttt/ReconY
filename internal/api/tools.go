package api

import (
	"net/http"
)

// ToolRegistryEntry describes a tool with its phase and I/O metadata.
type ToolRegistryEntry struct {
	Name        string   `json:"name"`
	Phase       int      `json:"phase"`
	PhaseName   string   `json:"phase_name"`
	Description string   `json:"description"`
	Available   bool     `json:"available"`
	Accepts     []string `json:"accepts"`
	Produces    []string `json:"produces"`
}

// toolMeta defines what each tool accepts and produces for interactive chaining.
var toolMeta = map[string]struct {
	Description string
	Accepts     []string
	Produces    []string
}{
	// Phase 1 - Passive
	"whois":        {Description: "Domain WHOIS record lookup", Accepts: []string{"domain"}, Produces: []string{"whois"}},
	"dns":          {Description: "DNS record resolution and enumeration", Accepts: []string{"domain"}, Produces: []string{"dns"}},
	"waybackurls":  {Description: "Fetch historical URLs from Wayback Machine", Accepts: []string{"domain"}, Produces: []string{"historical_urls"}},
	"gau":          {Description: "Fetch known URLs from AlienVault, Wayback, and Common Crawl", Accepts: []string{"domain"}, Produces: []string{"historical_urls"}},

	// Phase 2 - Subdomains
	"subfinder":    {Description: "Fast passive subdomain enumeration", Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"crtsh":        {Description: "Query Certificate Transparency logs for subdomains", Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"amass":        {Description: "In-depth network mapping and attack surface discovery", Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"puredns":      {Description: "Fast mass DNS resolution and brute-forcing", Accepts: []string{"domain", "subdomains"}, Produces: []string{"subdomains"}},

	// Phase 3 - Ports
	"nmap":         {Description: "Standard network mapper and port scanner", Accepts: []string{"subdomains"}, Produces: []string{"ports"}},
	"shodan":       {Description: "Search engine for Internet-connected devices", Accepts: []string{"subdomains"}, Produces: []string{"ports"}},
	"censys":       {Description: "Internet-wide scanner for hosts and certificates", Accepts: []string{"subdomains"}, Produces: []string{"ports"}},

	// Phase 4 - Fingerprint
	"httpx":        {Description: "Fast and multi-purpose HTTP toolkit", Accepts: []string{"subdomains"}, Produces: []string{"technologies", "urls"}},
	"waf_detect":   {Description: "Web Application Firewall detection", Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},
	"ssl_analyze":  {Description: "SSL/TLS certificate analysis", Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},
	"classify":     {Description: "Target classification based on HTTP response", Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},

	// Phase 5 - Content
	"katana":       {Description: "Next-generation crawling and spidering framework", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls", "parameters"}},
	"jsluice":      {Description: "Extract URLs, paths, and secrets from JavaScript", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls", "secrets"}},
	"secretfinder": {Description: "Discover sensitive data like API keys in JavaScript", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"secrets"}},
	"paramspider":  {Description: "Mine parameters from web archives", Accepts: []string{"domain", "subdomains"}, Produces: []string{"parameters"}},
	"ffuf":         {Description: "Fast web fuzzer for directory and file discovery", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls"}},
	"feroxbuster":  {Description: "Fast, simple, recursive content discovery", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls"}},
	"cmseek":       {Description: "CMS Detection and Exploitation suite", Accepts: []string{"domain", "subdomains"}, Produces: []string{"technologies"}},
	"gowitness":    {Description: "Web screenshot utility using Chrome Headless", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"screenshots"}},
	"static-analysis": {Description: "Perform static code analysis on web assets", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"secrets", "urls", "parameters"}},
	"ai_research":  {Description: "AI-assisted target research and profiling", Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"classifications"}},

	// Phase 6 - Cloud
	"bucket_enum":  {Description: "Enumerate public cloud storage buckets", Accepts: []string{"domain", "subdomains"}, Produces: []string{"cloud_assets"}},
	"gitdork":      {Description: "Search GitHub for sensitive information leaks", Accepts: []string{"domain"}, Produces: []string{"urls"}},
	"js_secrets":   {Description: "Scan JavaScript files for embedded secrets", Accepts: []string{"urls"}, Produces: []string{"secrets"}},

	// Phase 7 - Vulns
	"nuclei":       {Description: "Fast and customizable vulnerability scanner", Accepts: []string{"subdomains", "urls"}, Produces: []string{"vulnerabilities"}},
}

var phaseNames = map[int]string{
	1: "Passive Recon",
	2: "Subdomain Enumeration",
	3: "Port Scanning",
	4: "Fingerprinting",
	5: "Content Discovery",
	6: "Cloud & Secrets",
	7: "Vulnerability Scanning",
}

func (s *Server) getToolRegistry(w http.ResponseWriter, r *http.Request) {
	registered := s.Engine.RegisteredTools()
	checkResults := s.Engine.CheckTools()

	result := make(map[int][]ToolRegistryEntry)
	for phase, toolNames := range registered {
		for _, name := range toolNames {
			meta, ok := toolMeta[name]
			if !ok {
				meta = struct {
					Description string
					Accepts     []string
					Produces    []string
				}{Description: "Generic scanning tool", Accepts: []string{"domain"}, Produces: []string{}}
			}

			// Use the tool's own Check() method — internal tools return nil (available),
			// external tools check exec.LookPath
			available := checkResults[name] == nil

			result[phase] = append(result[phase], ToolRegistryEntry{
				Name:        name,
				Phase:       phase,
				PhaseName:   phaseNames[phase],
				Description: meta.Description,
				Available:   available,
				Accepts:     meta.Accepts,
				Produces:    meta.Produces,
			})
		}
	}

	writeJSON(w, 200, result)
}
