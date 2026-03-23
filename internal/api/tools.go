package api

import (
	"net/http"
)

// ToolRegistryEntry describes a tool with its phase and I/O metadata.
type ToolRegistryEntry struct {
	Name      string   `json:"name"`
	Phase     int      `json:"phase"`
	PhaseName string   `json:"phase_name"`
	Available bool     `json:"available"`
	Accepts   []string `json:"accepts"`
	Produces  []string `json:"produces"`
}

// toolMeta defines what each tool accepts and produces for interactive chaining.
var toolMeta = map[string]struct {
	Accepts  []string
	Produces []string
}{
	// Phase 1 - Passive
	"whois":        {Accepts: []string{"domain"}, Produces: []string{"whois"}},
	"dns":          {Accepts: []string{"domain"}, Produces: []string{"dns"}},
	"waybackurls":  {Accepts: []string{"domain"}, Produces: []string{"historical_urls"}},
	"gau":          {Accepts: []string{"domain"}, Produces: []string{"historical_urls"}},

	// Phase 2 - Subdomains
	"subfinder":    {Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"crtsh":        {Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"amass":        {Accepts: []string{"domain"}, Produces: []string{"subdomains"}},
	"puredns":      {Accepts: []string{"domain", "subdomains"}, Produces: []string{"subdomains"}},

	// Phase 3 - Ports
	"nmap":         {Accepts: []string{"subdomains"}, Produces: []string{"ports"}},
	"shodan":       {Accepts: []string{"subdomains"}, Produces: []string{"ports"}},
	"censys":       {Accepts: []string{"subdomains"}, Produces: []string{"ports"}},

	// Phase 4 - Fingerprint
	"httpx":        {Accepts: []string{"subdomains"}, Produces: []string{"technologies", "urls"}},
	"waf_detect":   {Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},
	"ssl_analyze":  {Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},
	"classify":     {Accepts: []string{"subdomains"}, Produces: []string{"classifications"}},

	// Phase 5 - Content
	"katana":       {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls", "parameters"}},
	"jsluice":      {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls", "secrets"}},
	"secretfinder": {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"secrets"}},
	"paramspider":  {Accepts: []string{"domain", "subdomains"}, Produces: []string{"parameters"}},
	"ffuf":         {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls"}},
	"feroxbuster":  {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"urls"}},
	"cmseek":       {Accepts: []string{"domain", "subdomains"}, Produces: []string{"technologies"}},
	"gowitness":    {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"screenshots"}},
	"static-analysis": {Accepts: []string{"domain", "subdomains", "urls"}, Produces: []string{"secrets", "urls", "parameters"}},

	// Phase 6 - Cloud
	"bucket_enum":  {Accepts: []string{"domain", "subdomains"}, Produces: []string{"cloud_assets"}},
	"gitdork":      {Accepts: []string{"domain"}, Produces: []string{"urls"}},
	"js_secrets":   {Accepts: []string{"urls"}, Produces: []string{"secrets"}},

	// Phase 7 - Vulns
	"nuclei":       {Accepts: []string{"subdomains", "urls"}, Produces: []string{"vulnerabilities"}},
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
					Accepts  []string
					Produces []string
				}{Accepts: []string{"domain"}, Produces: []string{}}
			}

			// Use the tool's own Check() method — internal tools return nil (available),
			// external tools check exec.LookPath
			available := checkResults[name] == nil

			result[phase] = append(result[phase], ToolRegistryEntry{
				Name:      name,
				Phase:     phase,
				PhaseName: phaseNames[phase],
				Available: available,
				Accepts:   meta.Accepts,
				Produces:  meta.Produces,
			})
		}
	}

	writeJSON(w, 200, result)
}
