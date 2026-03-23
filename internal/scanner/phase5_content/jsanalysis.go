package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"reconx/internal/config"
	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
	"reconx/internal/scanner"
	"reconx/internal/tools"
)

// JSluiceRunner extracts URLs and secrets from JavaScript files.
type JSluiceRunner struct{}

func (j *JSluiceRunner) Name() string         { return "jsluice" }
func (j *JSluiceRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (j *JSluiceRunner) Check() error          { return tools.CheckBinary("jsluice") }

func (j *JSluiceRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	baseTargets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	// Detect site type
	siteType := models.SiteTypeUnknown
	for _, c := range input.Classifications {
		if c.SiteType != models.SiteTypeUnknown {
			siteType = c.SiteType
			break
		}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Analyzing JS on %d targets (site_type=%s)", len(baseTargets), siteType))

	urlCount := 0
	secretCount := 0

	// Step 1: Run jsluice on base targets
	for _, target := range baseTargets {
		if err := ctx.Err(); err != nil {
			return err
		}
		u, s := j.analyzeTarget(ctx, target, sink)
		urlCount += u
		secretCount += s
	}

	// Step 2: Discover and analyze JS files (all site types, not just SPA)
	jsFiles := j.discoverJSFiles(ctx, baseTargets, input.Config, input.AuthSessions)
	if len(jsFiles) > 0 {
		sink.LogLine(ctx, "stdout", fmt.Sprintf("Discovered %d JS files to analyze", len(jsFiles)))
		for _, jsURL := range jsFiles {
			if err := ctx.Err(); err != nil {
				return err
			}
			u, s := j.analyzeTarget(ctx, jsURL, sink)
			urlCount += u
			secretCount += s
		}
	}

	// Step 3: Extract endpoints from HTML (especially useful for Classic/SSR)
	if siteType != models.SiteTypeAPI {
		htmlEndpoints := j.extractHTMLEndpoints(ctx, baseTargets, input.Config, input.AuthSessions)
		for _, ep := range htmlEndpoints {
			sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
				URL:    ep,
				Source: "html_extract",
			})
			urlCount++
		}
		if len(htmlEndpoints) > 0 {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("Extracted %d endpoints from HTML", len(htmlEndpoints)))
		}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("jsluice done: %d URLs, %d secrets extracted", urlCount, secretCount))
	return nil
}

// analyzeTarget runs jsluice urls + secrets on a single target. Returns (urlCount, secretCount).
func (j *JSluiceRunner) analyzeTarget(ctx context.Context, target string, sink engine.ResultSink) (int, int) {
	urlCount := 0
	secretCount := 0

	// jsluice urls mode
	result, err := tools.RunToolWithProxy(ctx, "jsluice", []string{"urls", "-R", target}, "", func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry struct {
				URL    string `json:"url"`
				Source string `json:"source"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.URL != "" {
				sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
					URL:    entry.URL,
					Source: "jsluice",
				})
				urlCount++
			}
		}
	})
	if err != nil || (result != nil && result.ExitCode != 0) {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("jsluice urls failed for %s", target))
	}

	// jsluice secrets mode
	result, err = tools.RunToolWithProxy(ctx, "jsluice", []string{"secrets", "-R", target}, "", func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry struct {
				Kind     string `json:"kind"`
				Data     string `json:"data"`
				Severity string `json:"severity"`
				Context  string `json:"context"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Data != "" {
				severity := models.SeverityMedium
				if entry.Severity != "" {
					severity = models.Severity(strings.ToLower(entry.Severity))
				}
				var ctxStr *string
				if entry.Context != "" {
					ctxStr = &entry.Context
				}
				sink.AddSecret(ctx, &models.Secret{
					SourceURL:  target,
					SecretType: entry.Kind,
					Value:      entry.Data,
					Context:    ctxStr,
					Source:     "jsluice",
					Severity:   severity,
				})
				secretCount++
			}
		}
	})
	if err != nil || (result != nil && result.ExitCode != 0) {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("jsluice secrets failed for %s", target))
	}

	return urlCount, secretCount
}

// discoverJSFiles fetches HTML from base targets and extracts .js file URLs.
func (j *JSluiceRunner) discoverJSFiles(ctx context.Context, baseTargets []string, cfg *config.Config, authSessions []*httpkit.AuthSession) []string {
	client := httpkit.NewClient(cfg)
	seen := make(map[string]bool)
	var jsURLs []string

	jsRe := regexp.MustCompile(`(?:src|href)=["']([^"']*\.js[^"']*)["']`)

	for _, target := range baseTargets {
		if err := ctx.Err(); err != nil {
			break
		}

		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
		for _, sess := range authSessions {
			sess.ApplyToRequest(req)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		resp.Body.Close()

		for _, match := range jsRe.FindAllStringSubmatch(string(body), -1) {
			jsURL := match[1]
			// Resolve relative URLs
			if strings.HasPrefix(jsURL, "//") {
				jsURL = "https:" + jsURL
			} else if strings.HasPrefix(jsURL, "/") {
				jsURL = target + jsURL
			} else if !strings.HasPrefix(jsURL, "http") {
				jsURL = target + "/" + jsURL
			}
			// Skip external analytics/tracking JS
			if strings.Contains(jsURL, "google-analytics") || strings.Contains(jsURL, "googletagmanager") ||
				strings.Contains(jsURL, "facebook.net") || strings.Contains(jsURL, "doubleclick") {
				continue
			}
			if !seen[jsURL] {
				seen[jsURL] = true
				jsURLs = append(jsURLs, jsURL)
			}
		}
	}

	return jsURLs
}

// extractHTMLEndpoints fetches HTML and extracts endpoints from forms, links, and inline JS.
func (j *JSluiceRunner) extractHTMLEndpoints(ctx context.Context, baseTargets []string, cfg *config.Config, authSessions []*httpkit.AuthSession) []string {
	client := httpkit.NewClient(cfg)
	seen := make(map[string]bool)
	var endpoints []string

	// Patterns to extract
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`<form[^>]+action=["']([^"']+)["']`),
		regexp.MustCompile(`<a[^>]+href=["']([^"'#][^"']*)["']`),
		regexp.MustCompile(`(?:fetch|axios\.(?:get|post|put|delete)|XMLHttpRequest)\s*\(\s*["']([^"']+)["']`),
		regexp.MustCompile(`["'](/(?:api|v[0-9]|graphql|rest|auth|login|register|admin|dashboard|search|upload|download)[^"']*?)["']`),
		regexp.MustCompile(`(?:url|endpoint|path|href|action|src)\s*[:=]\s*["']([/][^"']+)["']`),
	}

	for _, target := range baseTargets {
		if err := ctx.Err(); err != nil {
			break
		}

		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		for _, sess := range authSessions {
			sess.ApplyToRequest(req)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		resp.Body.Close()

		html := string(body)

		for _, re := range patterns {
			for _, match := range re.FindAllStringSubmatch(html, -1) {
				ep := match[1]
				// Resolve relative URLs
				if strings.HasPrefix(ep, "/") {
					ep = target + ep
				} else if !strings.HasPrefix(ep, "http") {
					continue // skip mailto:, javascript:, etc.
				}
				// Skip external, anchors, and assets
				if strings.Contains(ep, "google") || strings.Contains(ep, "facebook") ||
					strings.HasSuffix(ep, ".css") || strings.HasSuffix(ep, ".png") ||
					strings.HasSuffix(ep, ".jpg") || strings.HasSuffix(ep, ".svg") ||
					strings.HasSuffix(ep, ".ico") || strings.HasSuffix(ep, ".woff") {
					continue
				}
				if !seen[ep] {
					seen[ep] = true
					endpoints = append(endpoints, ep)
				}
			}
		}
	}

	return endpoints
}

// SecretFinderRunner uses secretfinder to find secrets in JS.
type SecretFinderRunner struct{}

func (s *SecretFinderRunner) Name() string         { return "secretfinder" }
func (s *SecretFinderRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (s *SecretFinderRunner) Check() error          { return tools.CheckBinary("secretfinder") }

func (s *SecretFinderRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		tools.RunToolWithProxy(ctx, "secretfinder", []string{"-i", target, "-o", "cli"}, input.ProxyURL, func(stream, line string) {
			sink.LogLine(ctx, stream, line)
			if stream == "stdout" && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "[") {
				sink.AddSecret(ctx, &models.Secret{
					SourceURL:  target,
					SecretType: "unknown",
					Value:      strings.TrimSpace(line),
					Source:     "secretfinder",
					Severity:   models.SeverityMedium,
				})
			}
		})
	}
	return nil
}
