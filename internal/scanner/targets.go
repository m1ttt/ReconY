package scanner

import (
	"reconx/internal/models"
)

// BuildHTTPTargets returns URLs for scanning, preferring verified data.
// Priority: classifications (httpx) → alive subdomains → all subdomains → domain fallback.
// When the user explicitly selects unverified subdomains, they still get scanned.
// Guaranteed to never return an empty slice.
func BuildHTTPTargets(subdomains []models.Subdomain, classifications []models.SiteClassification, domain string) []string {
	var urls []string
	seen := make(map[string]bool)

	// Prefer URLs from classifications (came from httpx/classify, known reachable)
	for _, c := range classifications {
		if c.URL != "" && !seen[c.URL] {
			seen[c.URL] = true
			urls = append(urls, c.URL)
		}
	}
	if len(urls) > 0 {
		return urls
	}

	// Try alive subdomains first
	for _, sub := range subdomains {
		if sub.IsAlive {
			for _, scheme := range []string{"https://", "http://"} {
				u := scheme + sub.Hostname
				if !seen[u] {
					seen[u] = true
					urls = append(urls, u)
				}
			}
		}
	}
	if len(urls) > 0 {
		return urls
	}

	// No alive subdomains — use ALL provided subdomains (user selected them explicitly)
	for _, sub := range subdomains {
		for _, scheme := range []string{"https://", "http://"} {
			u := scheme + sub.Hostname
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	if len(urls) > 0 {
		return urls
	}

	// Last resort: both schemes on the main domain
	return []string{"https://" + domain, "http://" + domain}
}
