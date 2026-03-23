package config

import (
	"os"
	"path/filepath"
)

// DefaultConfig returns the base configuration with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	reconxDir := filepath.Join(home, ".reconx")

	t := true
	f := false
	threads30 := 30
	threads50 := 50
	rateLimit150 := 150

	return &Config{
		General: GeneralConfig{
			DBPath:             filepath.Join(reconxDir, "reconx.db"),
			ScreenshotsDir:     filepath.Join(reconxDir, "screenshots"),
			SecListsPath:       "/opt/SecLists",
			MaxConcurrentTools: 5,
			DefaultWorkflow:    "full",
			APIListenAddr:      "127.0.0.1:8420",
			RateLimitPerHost:   10,
			MaxRetries:         3,
		},
		APIKeys: APIKeysConfig{},
		Proxy: ProxyConfig{
			RotateEveryN:     50,
			RotateInterval:   "5m",
			MullvadLocations: []string{"us", "de", "nl", "se", "ch", "gb", "jp"},
		},
		Tools: map[string]ToolConfig{
			"subfinder": {Enabled: &t, Threads: &threads30, Timeout: "10m"},
			"crtsh":     {Enabled: &t, Timeout: "5m"},
			"amass":     {Enabled: &f, Timeout: "30m"},
			"puredns":   {Enabled: &t, Timeout: "15m"},
			"nmap": {
				Enabled: &t,
				Timeout: "30m",
				Options: map[string]any{
					"ports":     "top-1000",
					"scan_type": "-sT",
					"timing":    "-T4",
				},
			},
			"shodan":       {Enabled: &f},
			"censys":       {Enabled: &f},
			"httpx":        {Enabled: &t, Threads: &threads50, Timeout: "10m"},
			"katana":       {Enabled: &t, Timeout: "10m", Options: map[string]any{"depth": 3, "headless": true, "js_crawl": true}},
			"ffuf":         {Enabled: &t, Threads: &threads50, Timeout: "50m", Options: map[string]any{"match_codes": []any{200, 201, 301, 302, 403}, "filter_codes": []any{404}, "follow_redirects": true, "auto_calibrate": true}},
			"feroxbuster":  {Enabled: &t, Threads: &threads50, Timeout: "60m", Options: map[string]any{"depth": 4, "status_codes": "200,201,301,302,307,308,401,403,405", "extract_links": true, "auto_tune": true, "collect_extensions": true, "collect_backups": true}},
			"paramspider":  {Enabled: &t, Timeout: "10m"},
			"cmseek":       {Enabled: &t, Timeout: "10m"},
			"gowitness":    {Enabled: &t, Timeout: "20m"},
			"nuclei":       {Enabled: &t, Timeout: "30m", RateLimit: &rateLimit150, Options: map[string]any{"severity": []any{"low", "medium", "high", "critical"}, "exclude_tags": []any{"dos"}}},
			"whois":        {Enabled: &t, Timeout: "2m"},
			"dns":          {Enabled: &t, Timeout: "5m"},
			"waybackurls":  {Enabled: &t, Timeout: "10m"},
			"gau":          {Enabled: &t, Timeout: "10m"},
			"jsluice":      {Enabled: &t, Timeout: "10m"},
			"secretfinder": {Enabled: &t, Timeout: "10m"},
			"gitdork":      {Enabled: &t, Timeout: "10m"},
		},
		Wordlists: WordlistsConfig{
			DNSQuick:      "Discovery/DNS/subdomains-top1million-5000.txt",
			DNSStandard:   "Discovery/DNS/subdomains-top1million-20000.txt",
			DNSAggressive: "Discovery/DNS/subdomains-top1million-110000.txt",

			WebQuick:      "Discovery/Web-Content/common.txt",
			WebStandard:   "Discovery/Web-Content/raft-medium-words.txt",
			WebAggressive: "Discovery/Web-Content/directory-list-2.3-medium.txt",

			APIEndpoints: "Discovery/Web-Content/api/api-endpoints.txt",
			APIWild:      "Discovery/Web-Content/api/api-seen-in-wild.txt",

			Params: "Discovery/Web-Content/burp-parameter-names.txt",

			CMSWordpress: "Discovery/Web-Content/CMS/WordPress.txt",
			CMSDrupal:    "Discovery/Web-Content/CMS/Drupal.txt",
			CMSJoomla:    "Discovery/Web-Content/CMS/Joomla.txt",

			TechPHP:  "Discovery/Web-Content/Programming-Language-Specific/Common-PHP-Filenames.txt",
			TechJava: "Discovery/Web-Content/Programming-Language-Specific/Java-Spring-Boot.txt",
			TechRoR:  "Discovery/Web-Content/Programming-Language-Specific/ror.txt",

			LFI:  "Fuzzing/LFI/LFI-Jhaddix.txt",
			XSS:  "Fuzzing/XSS/XSS-Polyglot-Ultimate-0xsobky.txt",
			SQLi: "Fuzzing/SQLi/Generic-SQLi.txt",
			SSRF: "Fuzzing/SSRF/ips.txt",
		},
	}
}
