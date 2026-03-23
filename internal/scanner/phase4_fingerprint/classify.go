package phase4

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// ClassifyRunner determines site type (SPA/SSR/Classic/API) and infrastructure type.
type ClassifyRunner struct{}

func (c *ClassifyRunner) Name() string         { return "classify" }
func (c *ClassifyRunner) Phase() engine.PhaseID { return engine.PhaseFingerprint }
func (c *ClassifyRunner) Check() error          { return nil }

func (c *ClassifyRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	var targets []string
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		targets = []string{input.Workspace.Domain}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Classifying %d targets", len(targets)))

	client := httpkit.NewClientWithRedirects(input.Config, 5)

	for _, host := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.classifyHost(ctx, client, host, input, input.AuthSessions, sink)
	}

	return nil
}

func (c *ClassifyRunner) classifyHost(ctx context.Context, client *httpkit.Client, host string, input *engine.PhaseInput, authSessions []*httpkit.AuthSession, sink engine.ResultSink) {
	url := "https://" + host
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	for _, sess := range authSessions {
		sess.ApplyToRequest(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Try HTTP
		url = "http://" + host
		req, _ = http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err = client.Do(req)
		if err != nil {
			return
		}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	resp.Body.Close()
	html := string(body)

	var subdomainID *string
	for _, sub := range input.Subdomains {
		if sub.Hostname == host {
			subdomainID = &sub.ID
			break
		}
	}

	// Classify site type
	siteType, evidence := classifySiteType(html, resp.Header)

	// Classify infrastructure
	infraType := classifyInfra(resp.Header, host, input.Ports)

	evidenceJSON, _ := json.Marshal(evidence)
	evidenceStr := string(evidenceJSON)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: type=%s infra=%s", host, siteType, infraType))

	infraStr := string(infraType)
	sink.AddSiteClassification(ctx, &models.SiteClassification{
		SubdomainID: subdomainID,
		URL:         url,
		SiteType:    siteType,
		InfraType:   &infraStr,
		Evidence:    &evidenceStr,
	})
}

func classifySiteType(html string, headers http.Header) (models.SiteType, map[string]any) {
	evidence := map[string]any{}
	lower := strings.ToLower(html)
	score := map[models.SiteType]int{}

	// SPA signals
	spaSignals := []struct {
		pattern string
		weight  int
		note    string
	}{
		{"<div id=\"root\"></div>", 3, "React root div"},
		{"<div id=\"app\"></div>", 3, "Vue/generic app div"},
		{"<div id=\"__next\"", 2, "Next.js"},
		{"<div id=\"__nuxt\"", 2, "Nuxt.js"},
		{"<div id=\"__svelte\"", 2, "SvelteKit"},
		{"/_next/static", 2, "Next.js static assets"},
		{"/static/js/main.", 2, "CRA build"},
		{"/_app.js", 1, "Next.js app bundle"},
		{"type=\"module\"", 1, "ES module script"},
		{".js\"></script>", 1, "JS bundle"},
		{"_next/static/chunks", 2, "Next.js client chunks"},
	}

	// SSR signals
	ssrSignals := []struct {
		pattern string
		weight  int
		note    string
	}{
		{"window.__NEXT_DATA__", 2, "Next.js SSR"},
		{"window.__NUXT__", 2, "Nuxt SSR"},
		{"data-reactroot", 1, "React SSR"},
		{"data-server-rendered", 2, "Vue SSR"},
	}

	// Classic signals
	classicSignals := []struct {
		pattern string
		weight  int
		note    string
	}{
		{"wp-content", 3, "WordPress"},
		{"wp-includes", 3, "WordPress"},
		{"joomla", 2, "Joomla"},
		{"drupal", 2, "Drupal"},
		{"<form", 1, "HTML forms"},
		{"<table", 1, "HTML tables"},
		{"<iframe", 1, "iframes"},
		{".php", 1, "PHP references"},
		{".asp", 1, "ASP references"},
	}

	// API signals
	apiSignals := []struct {
		pattern string
		weight  int
		note    string
	}{
		{"\"swagger\"", 3, "Swagger/OpenAPI"},
		{"\"openapi\"", 3, "OpenAPI spec"},
		{"application/json", 2, "JSON response"},
		{"{\"error\"", 2, "JSON error response"},
		{"{\"message\"", 1, "JSON message"},
	}

	var signals []string

	for _, sig := range spaSignals {
		if strings.Contains(lower, strings.ToLower(sig.pattern)) {
			score[models.SiteTypeSPA] += sig.weight
			signals = append(signals, "SPA:"+sig.note)
		}
	}
	for _, sig := range ssrSignals {
		if strings.Contains(lower, strings.ToLower(sig.pattern)) {
			score[models.SiteTypeSSR] += sig.weight
			signals = append(signals, "SSR:"+sig.note)
		}
	}
	for _, sig := range classicSignals {
		if strings.Contains(lower, strings.ToLower(sig.pattern)) {
			score[models.SiteTypeClassic] += sig.weight
			signals = append(signals, "Classic:"+sig.note)
		}
	}
	for _, sig := range apiSignals {
		if strings.Contains(lower, strings.ToLower(sig.pattern)) {
			score[models.SiteTypeAPI] += sig.weight
			signals = append(signals, "API:"+sig.note)
		}
	}

	// Content-Type header hint
	ct := headers.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		score[models.SiteTypeAPI] += 3
		signals = append(signals, "API:JSON content-type")
	}

	// Check if body is mostly empty (SPA with JS rendering)
	trimmed := strings.TrimSpace(html)
	if len(trimmed) < 500 && strings.Contains(lower, "<script") {
		score[models.SiteTypeSPA] += 2
		signals = append(signals, "SPA:minimal HTML with scripts")
	}

	// SSR has both SPA and server-rendered content
	if score[models.SiteTypeSPA] > 0 && len(html) > 5000 {
		score[models.SiteTypeSSR] += 1
	}

	evidence["signals"] = signals
	evidence["scores"] = score

	// Pick highest score
	best := models.SiteTypeUnknown
	bestScore := 0
	for st, s := range score {
		if s > bestScore {
			best = st
			bestScore = s
		}
	}

	// Hybrid detection: strong signals for BOTH SPA and SSR frameworks
	if score[models.SiteTypeSPA] >= 3 && score[models.SiteTypeSSR] >= 2 {
		best = models.SiteTypeHybrid
	} else if score[models.SiteTypeSPA] > 0 && score[models.SiteTypeSSR] > 0 {
		if score[models.SiteTypeSSR] >= score[models.SiteTypeSPA] {
			best = models.SiteTypeSSR
		} else {
			best = models.SiteTypeSPA
		}
	}

	return best, evidence
}

func classifyInfra(headers http.Header, host string, ports []models.Port) models.InfraType {
	// Check headers for managed platforms
	managedHeaders := map[string]string{
		"x-vercel-id":        "serverless",
		"x-amz-cf-id":        "serverless",
		"x-azure-ref":        "container",
		"x-powered-by-plesk": "bare_metal",
		"fly-request-id":     "container",
		"x-render-origin-server": "container",
	}

	for header, infra := range managedHeaders {
		if headers.Get(header) != "" {
			switch infra {
			case "serverless":
				return models.InfraTypeServerless
			case "container":
				return models.InfraTypeContainer
			case "bare_metal":
				return models.InfraTypeBareMetal
			}
		}
	}

	// Check DNS TTL for ephemeral/serverless hints
	_, err := net.LookupHost(host)
	if err == nil {
		// Can't easily get TTL from Go stdlib, but low port count suggests serverless
	}

	// Port count heuristic
	hostPorts := 0
	for _, p := range ports {
		if strings.Contains(p.IPAddress, host) || p.State == "open" {
			hostPorts++
		}
	}

	if hostPorts > 5 {
		return models.InfraTypeBareMetal
	}
	if hostPorts <= 2 {
		return models.InfraTypeServerless
	}

	return models.InfraTypeUnknown
}
