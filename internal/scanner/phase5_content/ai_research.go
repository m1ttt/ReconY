package phase5

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
)

// AIResearchRunner uses an external LangGraph+OpenAI microservice
// to perform deep research on targets found in previous phases.
type AIResearchRunner struct{}

func (r *AIResearchRunner) Name() string { return "ai_research" }

func (r *AIResearchRunner) Phase() engine.PhaseID { return engine.PhaseContent }

func (r *AIResearchRunner) Check() error { return nil }

type aiRequest struct {
	Query         string `json:"query"`
	OpenAIAPIKey  string `json:"openai_api_key,omitempty"`
	OpenAIBaseURL string `json:"openai_base_url,omitempty"`
	OpenAIModel   string `json:"openai_model,omitempty"`
	TavilyAPIKey  string `json:"tavily_api_key,omitempty"`
}

type aiResponse struct {
	Result string `json:"result"`
}

type researchTarget struct {
	URL string
}

type researchContext struct {
	Host           string   `json:"host"`
	URL            string   `json:"url"`
	RelatedURLs    []string `json:"related_urls,omitempty"`
	HistoricalURLs []string `json:"historical_urls,omitempty"`
	Technologies   []string `json:"technologies,omitempty"`
	SiteType       string   `json:"site_type,omitempty"`
	InfraType      string   `json:"infra_type,omitempty"`
	WAFDetected    string   `json:"waf_detected,omitempty"`
	CDNDetected    string   `json:"cdn_detected,omitempty"`
	SSLGrade       string   `json:"ssl_grade,omitempty"`
	DNSRecords     []string `json:"dns_records,omitempty"`
	WhoisSignals   []string `json:"whois_signals,omitempty"`
}

func (r *AIResearchRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := buildResearchTargets(input)
	if len(targets) == 0 {
		return fmt.Errorf("no targets available for AI research")
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Starting AI research on %d targets via local microservice...", len(targets)))

	client := &http.Client{Timeout: 5 * time.Minute}

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		targetContext := buildResearchContext(input, target.URL)
		query := buildResearchQuery(targetContext)

		reqBody, err := json.Marshal(aiRequest{
			Query:         query,
			OpenAIAPIKey:  input.Config.APIKeys.OpenAIKey,
			OpenAIBaseURL: input.Config.APIKeys.OpenAIBaseURL,
			OpenAIModel:   input.Config.APIKeys.OpenAIModel,
			TavilyAPIKey:  input.Config.APIKeys.TavilyKey,
		})
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to marshal request for %s: %v", target.URL, err))
			continue
		}

		aiURL := input.Config.APIKeys.AIServiceURL
		if aiURL == "" {
			aiURL = "http://localhost:8000"
		}
		endpoint := fmt.Sprintf("%s/research", aiURL)

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(reqBody))
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to create request for %s: %v", target.URL, err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to reach ai-service for %s. Is it running? Error: %v", target.URL, err))
			continue
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed reading ai-service response for %s: %v", target.URL, readErr))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("ai-service returned status %d for %s: %s", resp.StatusCode, target.URL, string(bodyBytes)))
			continue
		}

		var result aiResponse
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to decode ai-service response for %s: %v", target.URL, err))
			continue
		}

		evidence := result.Result
		err = sink.AddSiteClassification(ctx, &models.SiteClassification{
			URL:      target.URL,
			Evidence: &evidence,
			SiteType: models.SiteTypeUnknown,
		})
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to save classification for %s: %v", target.URL, err))
		} else {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("Successfully generated AI research for %s", target.URL))
		}
	}

	return nil
}

func buildResearchTargets(input *engine.PhaseInput) []researchTarget {
	seen := map[string]bool{}
	var targets []researchTarget

	addURL := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		targets = append(targets, researchTarget{URL: raw})
	}

	// Prefer the exact outputs produced by chained modules.
	for _, u := range input.DiscoveredURLs {
		addURL(u.URL)
	}
	for _, c := range input.Classifications {
		addURL(c.URL)
	}

	// Fall back to host-level targets only when there are no URLs to work from.
	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			if sub.Hostname == "" {
				continue
			}
			if sub.IsAlive {
				addURL("https://" + sub.Hostname)
				addURL("http://" + sub.Hostname)
			}
		}
	}

	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			if sub.Hostname == "" {
				continue
			}
			addURL("https://" + sub.Hostname)
			addURL("http://" + sub.Hostname)
		}
	}

	// Absolute last resort for standalone runs with no prior outputs.
	if len(targets) == 0 && input.Workspace != nil && input.Workspace.Domain != "" {
		for _, target := range scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain) {
			addURL(target)
		}
	}

	return targets
}

func buildResearchContext(input *engine.PhaseInput, rawURL string) researchContext {
	host := hostFromRawURL(rawURL)
	context := researchContext{
		Host: host,
		URL:  rawURL,
	}

	for _, u := range input.DiscoveredURLs {
		if sameHost(host, u.URL) {
			context.RelatedURLs = appendIfMissing(context.RelatedURLs, compactURL(u.URL))
		}
	}

	for _, u := range input.HistoricalURLs {
		if sameHost(host, u.URL) {
			context.HistoricalURLs = appendIfMissing(context.HistoricalURLs, compactURL(u.URL))
		}
	}

	for _, t := range input.Technologies {
		if sameHost(host, t.URL) {
			label := t.Name
			if t.Version != nil && *t.Version != "" {
				label += " " + *t.Version
			}
			if t.Category != nil && *t.Category != "" {
				label += " (" + *t.Category + ")"
			}
			context.Technologies = appendIfMissing(context.Technologies, label)
		}
	}

	for _, c := range input.Classifications {
		if !sameHost(host, c.URL) {
			continue
		}
		if context.SiteType == "" && c.SiteType != "" && c.SiteType != models.SiteTypeUnknown {
			context.SiteType = string(c.SiteType)
		}
		if context.InfraType == "" && c.InfraType != nil {
			context.InfraType = *c.InfraType
		}
		if context.WAFDetected == "" && c.WAFDetected != nil {
			context.WAFDetected = *c.WAFDetected
		}
		if context.CDNDetected == "" && c.CDNDetected != nil {
			context.CDNDetected = *c.CDNDetected
		}
		if context.SSLGrade == "" && c.SSLGrade != nil {
			context.SSLGrade = *c.SSLGrade
		}
	}

	for _, record := range input.DNSRecords {
		if record.Host == host || record.Host == stripWWW(host) || stripWWW(record.Host) == stripWWW(host) {
			label := fmt.Sprintf("%s %s", record.RecordType, record.Value)
			context.DNSRecords = appendIfMissing(context.DNSRecords, label)
		}
	}

	for _, record := range input.WhoisRecords {
		if !domainMatchesHost(record.Domain, host, input.Workspace.Domain) {
			continue
		}
		if record.Registrar != nil && *record.Registrar != "" {
			context.WhoisSignals = appendIfMissing(context.WhoisSignals, "Registrar: "+*record.Registrar)
		}
		if record.Org != nil && *record.Org != "" {
			context.WhoisSignals = appendIfMissing(context.WhoisSignals, "Organization: "+*record.Org)
		}
		if record.Country != nil && *record.Country != "" {
			context.WhoisSignals = appendIfMissing(context.WhoisSignals, "Country: "+*record.Country)
		}
		if record.ASN != nil && *record.ASN != "" {
			context.WhoisSignals = appendIfMissing(context.WhoisSignals, "ASN: "+*record.ASN)
		}
		if record.ASNOrg != nil && *record.ASNOrg != "" {
			context.WhoisSignals = appendIfMissing(context.WhoisSignals, "ASN Org: "+*record.ASNOrg)
		}
	}

	sort.Strings(context.RelatedURLs)
	sort.Strings(context.HistoricalURLs)
	sort.Strings(context.Technologies)
	sort.Strings(context.DNSRecords)
	sort.Strings(context.WhoisSignals)

	context.RelatedURLs = limitStrings(context.RelatedURLs, 12)
	context.HistoricalURLs = limitStrings(context.HistoricalURLs, 8)
	context.Technologies = limitStrings(context.Technologies, 10)
	context.DNSRecords = limitStrings(context.DNSRecords, 10)
	context.WhoisSignals = limitStrings(context.WhoisSignals, 8)

	return context
}

func buildResearchQuery(ctx researchContext) string {
	contextJSON, _ := json.Marshal(ctx)
	return fmt.Sprintf(
		"Research this authorized target for security-relevant external intelligence and return the module JSON schema only. "+
			"Prioritize the supplied recon context over generic domain fallback. "+
			"Target URL: %s. Host: %s. "+
			"Use the following prior module output as grounded context and pivot from it when searching: %s",
		ctx.URL,
		ctx.Host,
		string(contextJSON),
	)
}

func hostFromRawURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return raw
}

func sameHost(expectedHost string, raw string) bool {
	return stripWWW(hostFromRawURL(raw)) == stripWWW(expectedHost)
}

func stripWWW(host string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
}

func domainMatchesHost(domain string, host string, workspaceDomain string) bool {
	domain = stripWWW(domain)
	host = stripWWW(host)
	workspaceDomain = stripWWW(workspaceDomain)

	if domain == "" {
		return false
	}
	if domain == host || strings.HasSuffix(host, "."+domain) {
		return true
	}
	return workspaceDomain != "" && domain == workspaceDomain
}

func compactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if len(raw) > 180 {
		return raw[:177] + "..."
	}
	return raw
}

func appendIfMissing(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func limitStrings(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return items[:max]
}
