package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
	"reconx/internal/scanner"
)

// StaticAnalysisRunner performs deep static analysis of HTML and JS to discover
// endpoints without dynamic crawling. Pure Go implementation, no external binaries.
type StaticAnalysisRunner struct {
	authSessions []*httpkit.AuthSession // set per-Run for authenticated fetching
}

func (s *StaticAnalysisRunner) Name() string         { return "static-analysis" }
func (s *StaticAnalysisRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (s *StaticAnalysisRunner) Check() error          { return nil }

const (
	maxJSFiles       = 50
	maxJSBodyLen     = 2 * 1024 * 1024 // 2MB
	maxHTMLLen       = 1 * 1024 * 1024 // 1MB
	maxSourceMapLen  = 5 * 1024 * 1024 // 5MB — source maps are larger than JS
	maxStringLiteral = 200             // cap string literal results per file
)

type endpointMatch struct {
	url        string
	method     string // GET, POST, etc. or empty
	source     string // "js_fetch", "js_axios", "route_def", "webpack", "html_form", "graphql", "api_base", "data_attr", "source_map", "string_literal", "source_map_route"
	confidence int    // 1-100
}

func (s *StaticAnalysisRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)
	client := httpkit.NewClient(input.Config)
	s.authSessions = input.AuthSessions

	seen := make(map[string]bool)
	analyzedJS := make(map[string]bool) // JS files already fetched and analyzed
	totalEndpoints := 0
	totalParams := 0
	totalJS := 0

	// Collect all JS URLs from the DB (discovered by katana, ffuf, html_extract, etc.)
	dbJSSet := make(map[string]bool)
	var dbJSURLs []string
	for _, u := range input.DiscoveredURLs {
		if isJSURL(u.URL) && !dbJSSet[u.URL] {
			dbJSSet[u.URL] = true
			dbJSURLs = append(dbJSURLs, u.URL)
		}
	}
	// Also from historical URLs (waybackurls, gau) — old JS may still be accessible
	for _, u := range input.HistoricalURLs {
		if isJSURL(u.URL) && !dbJSSet[u.URL] {
			dbJSSet[u.URL] = true
			dbJSURLs = append(dbJSURLs, u.URL)
		}
	}

	if len(dbJSURLs) > 0 {
		sink.LogLine(ctx, "stdout", fmt.Sprintf("Found %d JS files from previous scans to analyze", len(dbJSURLs)))
	}

	// allExtractors runs all endpoint extractors on a body of code.
	allExtractors := func(body string) []endpointMatch {
		var m []endpointMatch
		m = append(m, extractFetchCalls(body)...)
		m = append(m, extractRouteDefinitions(body)...)
		m = append(m, extractWebpackChunkRoutes(body)...)
		m = append(m, extractAPIBaseURLs(body)...)
		m = append(m, extractGraphQLOps(body)...)
		return m
	}

	// Helper to analyze a single JS file and collect matches
	analyzeJS := func(jsURL string) []endpointMatch {
		if analyzedJS[jsURL] {
			return nil
		}
		analyzedJS[jsURL] = true
		jsBody, err := s.fetchBody(ctx, client, jsURL, maxJSBodyLen)
		if err != nil {
			return nil
		}
		totalJS++
		var matches []endpointMatch

		// Try source map reconstruction first
		sourceMapRef := extractSourceMapURL(jsBody)
		if sourceMapRef != "" {
			mapURL := resolveSourceMapURL(sourceMapRef, jsURL)
			corpus, sourceFiles, err := s.fetchAndParseSourceMap(ctx, client, mapURL)
			if err == nil && corpus != "" {
				sink.LogLine(ctx, "stdout", fmt.Sprintf("Reconstructed %d source files from %s", len(sourceFiles), filepath.Base(mapURL)))
				// Run ALL extractors on reconstructed (un-minified) source
				matches = append(matches, allExtractors(corpus)...)
				// Extract routes from source file paths (pages/api/users.ts → /api/users)
				matches = append(matches, routesFromSourcePaths(sourceFiles)...)
				// Extract secrets from reconstructed source
				extractSecretsFromJS(ctx, sink, corpus, jsURL)
			}
		}

		// Run extractors on the raw (minified) JS too
		matches = append(matches, allExtractors(jsBody)...)
		matches = append(matches, extractSourceMapPaths(jsBody)...)

		// String literal extraction — catches paths buried in minified code
		matches = append(matches, extractStringLiterals(jsBody)...)

		// Extract secrets from raw JS
		extractSecretsFromJS(ctx, sink, jsBody, jsURL)

		return matches
	}

	// Helper to resolve, dedup, and emit matches
	emitMatches := func(matches []endpointMatch, baseURL *url.URL) {
		for _, m := range matches {
			rawMatch := strings.TrimSpace(m.url)
			if isTemplateOrPlaceholderString(rawMatch) {
				continue
			}
			// Low-confidence literals like "/a/b" from minified bundles are usually noise.
			if m.source == "string_literal" && m.confidence <= 50 && isLowSignalPath(rawMatch) {
				continue
			}

			resolved := s.resolveURL(m.url, baseURL)
			if resolved == "" {
				continue
			}
			if isTemplateOrPlaceholderString(resolved) {
				continue
			}
			if isExternalDomain(resolved, baseURL.Host) {
				continue
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true

			sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
				URL:    resolved,
				Source: "static-analysis",
			})
			totalEndpoints++

			if parsedURL, err := url.Parse(resolved); err == nil {
				for paramName := range parsedURL.Query() {
					sink.AddParameter(ctx, &models.Parameter{
						URL:       resolved,
						Name:      paramName,
						ParamType: "query",
						Source:    "static-analysis",
					})
					totalParams++
				}
			}
		}
	}

	// Phase 1: Analyze each target's HTML + inline scripts + linked JS
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		baseURL, err := url.Parse(target)
		if err != nil {
			continue
		}

		htmlBody, err := s.fetchBody(ctx, client, target, maxHTMLLen)
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("static-analysis: failed to fetch %s: %v", target, err))
			continue
		}

		// Extract JS file URLs from HTML
		jsURLs := s.extractScriptSrcs(htmlBody, baseURL)
		if len(jsURLs) > maxJSFiles {
			jsURLs = jsURLs[:maxJSFiles]
		}

		// Collect matches from HTML
		var allMatches []endpointMatch
		allMatches = append(allMatches, extractFormActions(htmlBody)...)
		allMatches = append(allMatches, extractDataAttributes(htmlBody)...)

		// Fetch and analyze each JS from HTML
		for _, jsURL := range jsURLs {
			if err := ctx.Err(); err != nil {
				return err
			}
			allMatches = append(allMatches, analyzeJS(jsURL)...)
		}

		// Inline script analysis
		allMatches = append(allMatches, extractFetchCalls(htmlBody)...)
		allMatches = append(allMatches, extractRouteDefinitions(htmlBody)...)
		allMatches = append(allMatches, extractAPIBaseURLs(htmlBody)...)
		allMatches = append(allMatches, extractGraphQLOps(htmlBody)...)
		allMatches = append(allMatches, extractStringLiterals(htmlBody)...)

		// Next.js: extract __NEXT_DATA__ and fetch build manifests
		nextData := extractNextData(htmlBody)
		if nextData != nil {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("Found __NEXT_DATA__ (buildId=%s, page=%s)", nextData.BuildID, nextData.Page))
			allMatches = append(allMatches, nextData.toEndpoints()...)

			// Fetch _buildManifest.js — contains ALL pages in the app
			if nextData.BuildID != "" {
				manifestRoutes := s.fetchBuildManifest(ctx, client, baseURL, nextData.BuildID)
				if len(manifestRoutes) > 0 {
					sink.LogLine(ctx, "stdout", fmt.Sprintf("Extracted %d routes from _buildManifest.js", len(manifestRoutes)))
					allMatches = append(allMatches, manifestRoutes...)
				}
			}
		}

		// Extract secrets from HTML (inline scripts, comments, data attributes)
		extractSecretsFromJS(ctx, sink, htmlBody, target)

		emitMatches(allMatches, baseURL)
	}

	// Phase 2: Analyze JS files from DB that we didn't already cover from HTML
	// These are JS discovered by katana, ffuf, waybackurls, gau, etc.
	extraJS := 0
	for _, jsURL := range dbJSURLs {
		if analyzedJS[jsURL] {
			// Already analyzed from HTML script tags — but we added it to analyzedJS above
			// so this only skips if it was found in extractScriptSrcs
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		parsed, err := url.Parse(jsURL)
		if err != nil {
			continue
		}

		matches := analyzeJS(jsURL)
		if len(matches) > 0 {
			baseURL := &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}
			emitMatches(matches, baseURL)
			extraJS++
		}
	}

	// Phase 3: Also analyze DB JS that were in analyzedJS (from HTML) — they were skipped
	// because we already fetched them in Phase 1. Instead, process the DB JS that
	// were NOT in the HTML. We need to re-check: analyzedJS was set for HTML JS *before*
	// the DB loop, so DB JS that match HTML JS correctly skip. Only truly new DB JS run.
	// This is already handled above — the continue triggers on analyzedJS entries set by extractScriptSrcs.

	sink.LogLine(ctx, "stdout", fmt.Sprintf(
		"static-analysis done: %d endpoints, %d params from %d JS files (%d from HTML, %d from DB) across %d targets",
		totalEndpoints, totalParams, totalJS, totalJS-extraJS, extraJS, len(targets)))
	return nil
}

// isJSURL returns true if the URL looks like a JavaScript file.
func isJSURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	// Remove query string for extension check
	if idx := strings.IndexByte(lower, '?'); idx != -1 {
		lower = lower[:idx]
	}
	return strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".mjs") ||
		strings.HasSuffix(lower, ".jsx") ||
		strings.Contains(lower, ".js?") ||
		strings.Contains(lower, "/chunks/") ||
		strings.Contains(lower, "/_next/static/")
}

// fetchBody performs a GET request and returns the response body as a string, limited to maxLen bytes.
func (s *StaticAnalysisRunner) fetchBody(ctx context.Context, client *httpkit.Client, targetURL string, maxLen int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	for _, sess := range s.authSessions {
		sess.ApplyToRequest(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLen))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractScriptSrcs parses HTML for <script src="..."> tags and returns absolute URLs.
func (s *StaticAnalysisRunner) extractScriptSrcs(html string, base *url.URL) []string {
	re := regexp.MustCompile(`<script[^>]+src=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	var urls []string
	for _, m := range matches {
		resolved := s.resolveURL(m[1], base)
		if resolved == "" || seen[resolved] {
			continue
		}
		seen[resolved] = true
		urls = append(urls, resolved)
	}
	return urls
}

// resolveURL resolves a potentially relative URL against a base URL.
// Returns empty string if the URL is invalid or useless (javascript:, mailto:, etc.).
func (s *StaticAnalysisRunner) resolveURL(raw string, base *url.URL) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "#" || raw == "/" {
		return ""
	}
	if isTemplateOrPlaceholderString(raw) {
		return ""
	}
	// Skip non-http schemes
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") ||
		strings.HasPrefix(lower, "tel:") {
		return ""
	}

	// Handle protocol-relative URLs
	if strings.HasPrefix(raw, "//") {
		raw = base.Scheme + ":" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(parsed)
	// Only keep http/https
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

// isExternalDomain returns true for known external/tracking domains or domains
// that don't share a suffix with the target host.
var externalDomains = []string{
	"google.com", "googleapis.com", "gstatic.com", "google-analytics.com",
	"googletagmanager.com", "googlesyndication.com", "doubleclick.net",
	"facebook.com", "facebook.net", "fbcdn.net",
	"twitter.com", "twimg.com",
	"cloudflare.com", "cdnjs.cloudflare.com",
	"jsdelivr.net", "unpkg.com",
	"jquery.com", "bootstrapcdn.com",
	"fontawesome.com", "fonts.googleapis.com",
	"youtube.com", "ytimg.com",
	"linkedin.com", "instagram.com",
	"amazon.com", "amazonaws.com",
	"hotjar.com", "intercom.io", "zendesk.com",
	"sentry.io", "segment.io", "segment.com",
	"mixpanel.com", "amplitude.com",
	"stripe.com", "paypal.com",
}

func isExternalDomain(rawURL string, targetHost string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	host := strings.ToLower(parsed.Hostname())

	for _, ext := range externalDomains {
		if host == ext || strings.HasSuffix(host, "."+ext) {
			return true
		}
	}

	// If target host is a subdomain of example.com, allow any *.example.com
	targetBase := extractBaseDomain(targetHost)
	urlBase := extractBaseDomain(host)
	return targetBase != urlBase
}

// extractBaseDomain returns the last two segments of a hostname (e.g. "example.com" from "sub.example.com").
func extractBaseDomain(host string) string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return host
}

// --- Extractors ---

// extractFetchCalls extracts endpoints from fetch() and axios calls.
var reFetchCalls = regexp.MustCompile(
	`(?:fetch|axios)\s*(?:\.\s*(?:get|post|put|delete|patch|head|options))?\s*\(\s*["'` + "`" + `]([^"'` + "`" + `\s]{2,})["'` + "`" + `]`,
)

var reAxiosMethods = regexp.MustCompile(
	`axios\s*\.\s*(get|post|put|delete|patch|head|options)\s*\(\s*["'` + "`" + `]([^"'` + "`" + `\s]{2,})["'` + "`" + `]`,
)

func extractFetchCalls(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	// axios with explicit method
	for _, m := range reAxiosMethods.FindAllStringSubmatch(js, -1) {
		ep := m[2]
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     strings.ToUpper(m[1]),
			source:     "js_axios",
			confidence: 85,
		})
	}

	// Generic fetch calls
	for _, m := range reFetchCalls.FindAllStringSubmatch(js, -1) {
		ep := m[1]
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     "",
			source:     "js_fetch",
			confidence: 80,
		})
	}

	return results
}

// extractRouteDefinitions extracts route paths from React Router, Express, and Next.js patterns.
var reRoutePatterns = []*regexp.Regexp{
	// React Router: path: "/...", path="/..."
	regexp.MustCompile(`path\s*[:=]\s*["'](/[^"']+)["']`),
	// Express-style: app.get("/...", ...), router.post("/...", ...)
	regexp.MustCompile(`(?:app|router)\s*\.\s*(?:get|post|put|delete|patch|all|use)\s*\(\s*["'](/[^"']+)["']`),
	// Next.js dynamic routes in chunk manifests
	regexp.MustCompile(`["'](/(?:api|pages?|app)/[^"']+)["']`),
}

func extractRouteDefinitions(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, re := range reRoutePatterns {
		for _, m := range re.FindAllStringSubmatch(js, -1) {
			ep := m[1]
			if seen[ep] {
				continue
			}
			seen[ep] = true
			results = append(results, endpointMatch{
				url:        ep,
				method:     "",
				source:     "route_def",
				confidence: 70,
			})
		}
	}
	return results
}

// extractWebpackChunkRoutes extracts paths from webpack chunk manifests.
var reWebpackPatterns = []*regexp.Regexp{
	// "chunkName": "/path"
	regexp.MustCompile(`["']chunkName["']\s*:\s*["'](/[^"']+)["']`),
	// Webpack chunk map: 123: "/path"
	regexp.MustCompile(`\d+\s*:\s*["'](/[a-zA-Z][^"']{2,})["']`),
	// __webpack_require__ path references
	regexp.MustCompile(`["']\.(/[^"']+\.(?:js|mjs|jsx|ts|tsx))["']`),
}

func extractWebpackChunkRoutes(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, re := range reWebpackPatterns {
		for _, m := range re.FindAllStringSubmatch(js, 100) { // limit per pattern
			ep := m[1]
			if seen[ep] {
				continue
			}
			seen[ep] = true
			results = append(results, endpointMatch{
				url:        ep,
				method:     "",
				source:     "webpack",
				confidence: 50,
			})
		}
	}
	return results
}

// extractAPIBaseURLs extracts base URL definitions from JS.
var reAPIBase = regexp.MustCompile(
	`(?:baseURL|BASE_URL|API_URL|apiEndpoint|API_BASE|api_base|apiUrl|apiBase|API_ENDPOINT|API_HOST)\s*[:=]\s*["'` + "`" + `](https?://[^"'` + "`" + `\s]+|/[^"'` + "`" + `\s]+)["'` + "`" + `]`,
)

func extractAPIBaseURLs(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, m := range reAPIBase.FindAllStringSubmatch(js, -1) {
		ep := m[1]
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     "",
			source:     "api_base",
			confidence: 90,
		})
	}
	return results
}

// extractGraphQLOps extracts GraphQL query/mutation operation names and endpoints.
var reGraphQLPatterns = []*regexp.Regexp{
	// gql`query ... { or gql`mutation ... {
	regexp.MustCompile("(?:gql|graphql)\\s*`[^`]*?(query|mutation)\\s+(\\w+)"),
	// Inline: query { ... }, mutation { ... }
	regexp.MustCompile(`["']((?:query|mutation)\s+\w+\s*(?:\([^)]*\))?\s*\{)`),
	// GraphQL endpoint references
	regexp.MustCompile(`["'](/graphql[^"']*)["']`),
}

func extractGraphQLOps(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, re := range reGraphQLPatterns {
		for _, m := range re.FindAllStringSubmatch(js, -1) {
			var ep string
			if len(m) > 2 && m[2] != "" {
				// Named operation: generate a synthetic graphql endpoint
				ep = "/graphql"
			} else {
				ep = m[1]
			}
			if seen[ep] {
				continue
			}
			seen[ep] = true
			results = append(results, endpointMatch{
				url:        ep,
				method:     "POST",
				source:     "graphql",
				confidence: 75,
			})
		}
	}
	return results
}

// extractFormActions extracts endpoints from <form action="..."> tags.
var reFormAction = regexp.MustCompile(`<form[^>]*\saction=["']([^"']+)["'][^>]*>`)
var reFormMethod = regexp.MustCompile(`<form[^>]*\smethod=["']([^"']+)["'][^>]*\saction=["']([^"']+)["']|<form[^>]*\saction=["']([^"']+)["'][^>]*\smethod=["']([^"']+)["']`)

func extractFormActions(html string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, m := range reFormMethod.FindAllStringSubmatch(html, -1) {
		var method, ep string
		if m[1] != "" {
			method = strings.ToUpper(m[1])
			ep = m[2]
		} else {
			ep = m[3]
			method = strings.ToUpper(m[4])
		}
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     method,
			source:     "html_form",
			confidence: 95,
		})
	}

	// Also catch forms without explicit method (default GET)
	for _, m := range reFormAction.FindAllStringSubmatch(html, -1) {
		ep := m[1]
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     "GET",
			source:     "html_form",
			confidence: 95,
		})
	}

	return results
}

// extractDataAttributes extracts URLs from data-url, data-action, data-endpoint, data-href attributes.
var reDataAttrs = regexp.MustCompile(`data-(?:url|action|endpoint|href|api|src)\s*=\s*["']([^"']+)["']`)

func extractDataAttributes(html string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, m := range reDataAttrs.FindAllStringSubmatch(html, -1) {
		ep := m[1]
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     "",
			source:     "data_attr",
			confidence: 80,
		})
	}
	return results
}

// extractSourceMapPaths extracts source map file paths from JS files.
var reSourceMap = regexp.MustCompile(`//[#@]\s*sourceMappingURL=(\S+)`)

func extractSourceMapPaths(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, m := range reSourceMap.FindAllStringSubmatch(js, -1) {
		ep := m[1]
		// Skip data: URIs
		if strings.HasPrefix(ep, "data:") {
			continue
		}
		if seen[ep] {
			continue
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			method:     "",
			source:     "source_map",
			confidence: 60,
		})
	}
	return results
}

// --- Next.js Extraction ---

// nextDataPayload holds parsed __NEXT_DATA__ fields.
type nextDataPayload struct {
	BuildID   string         `json:"buildId"`
	Page      string         `json:"page"`
	Query     map[string]any `json:"query"`
	Props     map[string]any `json:"props"`
	RuntimeConfig map[string]any `json:"runtimeConfig"`
}

// toEndpoints converts __NEXT_DATA__ fields to endpoint matches.
func (nd *nextDataPayload) toEndpoints() []endpointMatch {
	var results []endpointMatch

	// The current page is a route
	if nd.Page != "" && nd.Page != "/" {
		results = append(results, endpointMatch{
			url:        nd.Page,
			source:     "next_data",
			confidence: 95,
		})
	}

	// Extract API routes and URLs from props (recursive)
	for _, v := range extractURLsFromJSON(nd.Props) {
		results = append(results, endpointMatch{
			url:        v,
			source:     "next_data",
			confidence: 75,
		})
	}

	// Extract from runtimeConfig (often has API base URLs)
	for _, v := range extractURLsFromJSON(nd.RuntimeConfig) {
		results = append(results, endpointMatch{
			url:        v,
			source:     "next_data",
			confidence: 85,
		})
	}

	return results
}

// reNextDataScript matches <script id="__NEXT_DATA__" ...>{...}</script>
var reNextDataScript = regexp.MustCompile(`<script\s+id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// reNextDataWindow matches window.__NEXT_DATA__={...}
var reNextDataWindow = regexp.MustCompile(`__NEXT_DATA__\s*=\s*(\{.*?\})\s*[;<]`)

// extractNextData parses __NEXT_DATA__ JSON from HTML.
func extractNextData(html string) *nextDataPayload {
	var jsonStr string

	// Try <script id="__NEXT_DATA__">
	if m := reNextDataScript.FindStringSubmatch(html); m != nil {
		jsonStr = m[1]
	} else if m := reNextDataWindow.FindStringSubmatch(html); m != nil {
		jsonStr = m[1]
	}

	if jsonStr == "" {
		return nil
	}

	var nd nextDataPayload
	if err := json.Unmarshal([]byte(jsonStr), &nd); err != nil {
		return nil
	}

	return &nd
}

// extractURLsFromJSON recursively extracts path-like and URL-like strings from a JSON structure.
func extractURLsFromJSON(data map[string]any) []string {
	var results []string
	seen := make(map[string]bool)

	var walk func(v any)
	walk = func(v any) {
		switch val := v.(type) {
		case string:
			if isAPILikePath(val) && !seen[val] {
				seen[val] = true
				results = append(results, val)
			}
		case map[string]any:
			for _, child := range val {
				walk(child)
			}
		case []any:
			for _, child := range val {
				walk(child)
			}
		}
	}

	if data != nil {
		walk(data)
	}
	return results
}

// isAPILikePath returns true if a string looks like an API path or internal URL.
func isAPILikePath(s string) bool {
	if len(s) < 4 || strings.ContainsRune(s, ' ') {
		return false
	}
	// Full URL
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		// Skip common external URLs
		lower := strings.ToLower(s)
		for _, ext := range externalDomains {
			if strings.Contains(lower, ext) {
				return false
			}
		}
		return true
	}
	// Path starting with /
	if strings.HasPrefix(s, "/") && len(s) > 1 {
		// Skip static assets
		return isValidStringLiteral(s)
	}
	// API keyword paths
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "api/") || strings.HasPrefix(lower, "v1/") || strings.HasPrefix(lower, "v2/") || strings.HasPrefix(lower, "graphql")
}

// reBuildManifestRoutes matches route keys in _buildManifest.js
// Format: self.__BUILD_MANIFEST={"/":["chunks/..."],"/about":["chunks/..."],"/api/users":["chunks/..."]}
// Or: "/dashboard":[...], "/settings":[...] in the JS object
var reBuildManifestRoutes = regexp.MustCompile(`"(/[^"]*)":\s*\[`)

// fetchBuildManifest fetches /_next/static/{buildId}/_buildManifest.js and extracts all page routes.
func (s *StaticAnalysisRunner) fetchBuildManifest(ctx context.Context, client *httpkit.Client, baseURL *url.URL, buildID string) []endpointMatch {
	manifestURL := fmt.Sprintf("%s://%s/_next/static/%s/_buildManifest.js", baseURL.Scheme, baseURL.Host, buildID)

	body, err := s.fetchBody(ctx, client, manifestURL, maxJSBodyLen)
	if err != nil {
		return nil
	}

	return parseBuildManifest(body)
}

// parseBuildManifest extracts routes from a _buildManifest.js body.
func parseBuildManifest(body string) []endpointMatch {
	matches := reBuildManifestRoutes.FindAllStringSubmatch(body, -1)
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, m := range matches {
		route := m[1]
		if route == "/" || seen[route] {
			continue
		}
		// Skip Next.js internal routes
		if strings.HasPrefix(route, "/_") {
			continue
		}
		seen[route] = true
		results = append(results, endpointMatch{
			url:        route,
			source:     "next_manifest",
			confidence: 95, // very high — these are actual app routes
		})
	}
	return results
}

// --- Source Map Reconstruction ---

// extractSourceMapURL finds the sourceMappingURL reference in JS content.
func extractSourceMapURL(js string) string {
	m := reSourceMap.FindStringSubmatch(js)
	if m == nil {
		return ""
	}
	ref := m[1]
	if strings.HasPrefix(ref, "data:") {
		return ""
	}
	return ref
}

// resolveSourceMapURL resolves a source map URL relative to the JS file URL.
func resolveSourceMapURL(mapRef, jsURL string) string {
	// Absolute URL
	if strings.HasPrefix(mapRef, "http://") || strings.HasPrefix(mapRef, "https://") {
		return mapRef
	}
	// Relative: resolve against JS file URL
	parsed, err := url.Parse(jsURL)
	if err != nil {
		return mapRef
	}
	ref, err := url.Parse(mapRef)
	if err != nil {
		return mapRef
	}
	return parsed.ResolveReference(ref).String()
}

// sourceMapJSON represents the structure of a .js.map file.
type sourceMapJSON struct {
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent"`
}

// fetchAndParseSourceMap fetches a .js.map file, parses it, and returns:
// - corpus: all reconstructed source files joined together
// - sourceFiles: list of source file paths (for route extraction)
func (s *StaticAnalysisRunner) fetchAndParseSourceMap(ctx context.Context, client *httpkit.Client, mapURL string) (string, []string, error) {
	body, err := s.fetchBody(ctx, client, mapURL, maxSourceMapLen)
	if err != nil {
		return "", nil, err
	}

	var sm sourceMapJSON
	if err := json.Unmarshal([]byte(body), &sm); err != nil {
		return "", nil, fmt.Errorf("parsing source map: %w", err)
	}

	// Reconstruct corpus from sourcesContent
	var parts []string
	for i, content := range sm.SourcesContent {
		if content == "" {
			continue
		}
		var label string
		if i < len(sm.Sources) {
			label = sm.Sources[i]
		}
		parts = append(parts, fmt.Sprintf("// === Source: %s ===\n%s", label, content))
	}

	// Filter source paths (remove node_modules, webpack internals)
	var sourceFiles []string
	for _, src := range sm.Sources {
		cleaned := cleanSourcePath(src)
		if cleaned == "" {
			continue
		}
		sourceFiles = append(sourceFiles, cleaned)
	}

	return strings.Join(parts, "\n\n"), sourceFiles, nil
}

// cleanSourcePath strips webpack prefixes and filters out non-app sources.
func cleanSourcePath(src string) string {
	// Strip webpack:// and similar prefixes
	src = strings.TrimPrefix(src, "webpack:///")
	src = strings.TrimPrefix(src, "webpack://")
	src = strings.TrimPrefix(src, "webpack-internal:///")
	src = strings.TrimPrefix(src, "webpack-internal://")
	src = strings.TrimPrefix(src, "./")
	src = strings.TrimPrefix(src, "../")
	src = strings.TrimPrefix(src, "/")

	// Skip non-application sources
	lower := strings.ToLower(src)
	if strings.Contains(lower, "node_modules/") ||
		strings.Contains(lower, "__webpack/") ||
		strings.HasPrefix(lower, "webpack/") ||
		strings.HasPrefix(lower, "(webpack)/") ||
		lower == "" {
		return ""
	}

	return src
}

// routesFromSourcePaths derives URL routes from source file paths.
// E.g., "pages/api/users.ts" → "/api/users", "src/routes/dashboard.tsx" → "/dashboard"
func routesFromSourcePaths(sourceFiles []string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	for _, src := range sourceFiles {
		route := sourcePathToRoute(src)
		if route == "" || seen[route] {
			continue
		}
		seen[route] = true
		results = append(results, endpointMatch{
			url:        route,
			method:     "",
			source:     "source_map_route",
			confidence: 65,
		})
	}
	return results
}

// sourcePathToRoute converts a source file path to a URL route.
func sourcePathToRoute(src string) string {
	// Next.js: pages/api/users.ts → /api/users
	// Next.js: app/dashboard/page.tsx → /dashboard
	// Generic: src/routes/login.tsx → /login

	// Try known patterns
	patterns := []struct {
		prefix string
		strip  bool // strip "page" suffix for App Router
	}{
		{"pages/", false},
		{"app/", true},
		{"src/pages/", false},
		{"src/routes/", false},
		{"src/views/", false},
	}

	for _, p := range patterns {
		idx := strings.Index(src, p.prefix)
		if idx == -1 {
			continue
		}
		route := src[idx+len(p.prefix):]

		// Remove file extension
		ext := filepath.Ext(route)
		if ext != "" {
			route = route[:len(route)-len(ext)]
		}

		// Remove index/page suffixes — these map to the directory root
		route = strings.TrimSuffix(route, "/index")
		route = strings.TrimSuffix(route, "/page")
		if route == "index" || route == "page" {
			return "" // root index → skip
		}

		// Skip _app, _document, _error (Next.js internal)
		base := filepath.Base(route)
		if strings.HasPrefix(base, "_") {
			return ""
		}

		// Clean up and add leading slash
		route = "/" + route
		route = strings.ReplaceAll(route, "//", "/")

		if route == "/" {
			return ""
		}
		return route
	}

	return ""
}

// --- String Literal Extraction ---

// reStringPath matches quoted strings that look like URL paths.
var reStringPath = regexp.MustCompile(`["'](/[a-zA-Z][a-zA-Z0-9/_\-.]{2,100})["']`)

// reStringURL matches quoted full HTTP/HTTPS URLs.
var reStringURL = regexp.MustCompile(`["'](https?://[^"'\s]{5,200})["']`)

// reStringAPI matches quoted strings starting with common API keywords.
var reStringAPI = regexp.MustCompile(`["']((?:api|v[0-9]|rest|graphql|auth|oauth|admin|login|logout|register|signup|profile|settings|account|users?|posts?|comments?|search|upload|download|webhook|callback|notify|status|health)/[a-zA-Z0-9/_\-.]+)["']`)

// Asset extensions to skip in string literal extraction.
var assetExtensions = map[string]bool{
	".js": true, ".mjs": true, ".jsx": true, ".ts": true, ".tsx": true,
	".css": true, ".scss": true, ".less": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".webp": true, ".avif": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	".map": true,
	".mp3": true, ".mp4": true, ".webm": true,
}

// Framework paths to skip.
var frameworkPrefixes = []string{
	"/_next/", "/__webpack/", "/node_modules/", "/static/chunks/",
	"/static/css/", "/static/media/", "/_buildManifest", "/_ssgManifest",
	"/webpack", "/favicon", "/manifest",
}

// reWebpackHash detects paths ending in hex hashes (e.g. /abc1234def5678).
var reWebpackHash = regexp.MustCompile(`/[a-f0-9]{8,}$`)

func extractStringLiterals(js string) []endpointMatch {
	var results []endpointMatch
	seen := make(map[string]bool)

	addMatch := func(ep string, confidence int) {
		if seen[ep] || len(results) >= maxStringLiteral {
			return
		}
		if !isValidStringLiteral(ep) {
			return
		}
		seen[ep] = true
		results = append(results, endpointMatch{
			url:        ep,
			source:     "string_literal",
			confidence: confidence,
		})
	}

	// Path-like strings: "/api/v1/users"
	for _, m := range reStringPath.FindAllStringSubmatch(js, -1) {
		addMatch(m[1], 50)
	}

	// Full URL strings: "https://api.example.com/v1"
	for _, m := range reStringURL.FindAllStringSubmatch(js, -1) {
		addMatch(m[1], 55)
	}

	// API keyword paths: "api/v2/products"
	for _, m := range reStringAPI.FindAllStringSubmatch(js, -1) {
		addMatch(m[1], 45)
	}

	return results
}

// isValidStringLiteral filters out noise from string literal extraction.
func isValidStringLiteral(s string) bool {
	// Too short
	if len(s) < 4 {
		return false
	}

	// Contains spaces (not a path)
	if strings.ContainsRune(s, ' ') {
		return false
	}
	// Dynamic template placeholders (e.g., ${id}, {{id}}) are not concrete routes.
	if isTemplateOrPlaceholderString(s) {
		return false
	}

	// Skip asset extensions
	lower := strings.ToLower(s)
	dotIdx := strings.LastIndexByte(lower, '.')
	if dotIdx != -1 {
		ext := lower[dotIdx:]
		if assetExtensions[ext] {
			return false
		}
	}

	// Skip framework internal paths
	for _, prefix := range frameworkPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}

	// Skip webpack hash paths
	if reWebpackHash.MatchString(lower) {
		return false
	}

	return true
}

func isTemplateOrPlaceholderString(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "${") ||
		strings.Contains(lower, "{{") ||
		strings.Contains(lower, "}}") ||
		strings.Contains(lower, "%7b") ||
		strings.Contains(lower, "%7d")
}

func isLowSignalPath(s string) bool {
	if !strings.HasPrefix(s, "/") {
		return false
	}
	trim := strings.TrimPrefix(s, "/")
	if trim == "" {
		return true
	}
	parts := strings.Split(trim, "/")
	if len(parts) == 0 {
		return true
	}
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Treat dynamic segments as potentially valid.
		if strings.HasPrefix(p, ":") || strings.HasPrefix(p, "[") {
			return false
		}
		if len(p) > 1 {
			return false
		}
	}
	return true
}

// --- Secret Detection ---

type secretPattern struct {
	name     string
	re       *regexp.Regexp
	severity models.Severity
	category string // "secret", "internal_url", "auth_storage", "debug"
}

var secretPatterns = []secretPattern{
	// API keys and secrets
	{name: "AWS Access Key", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), severity: models.SeverityCritical, category: "secret"},
	{name: "JWT Token", re: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9._-]{10,}\.[A-Za-z0-9._-]{10,}`), severity: models.SeverityHigh, category: "secret"},
	{name: "Bearer Token", re: regexp.MustCompile(`[Bb]earer\s+[A-Za-z0-9\-._~+/]{20,}=*`), severity: models.SeverityHigh, category: "secret"},
	{name: "Stripe Secret Key", re: regexp.MustCompile(`sk_live_[A-Za-z0-9]{20,}`), severity: models.SeverityCritical, category: "secret"},
	{name: "Stripe Publishable Key", re: regexp.MustCompile(`pk_live_[A-Za-z0-9]{20,}`), severity: models.SeverityLow, category: "secret"},
	{name: "Generic Secret", re: regexp.MustCompile(`(?:api_key|secret_key|client_secret|private_key|access_key|auth_token|jwt_secret|api_secret)\s*[:=]\s*["']([^"']{8,})["']`), severity: models.SeverityHigh, category: "secret"},
	{name: "Private Key Block", re: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA )?PRIVATE KEY-----`), severity: models.SeverityCritical, category: "secret"},

	// Internal URLs
	{name: "Localhost URL", re: regexp.MustCompile(`https?://localhost[:/][^"'\s]{2,}`), severity: models.SeverityMedium, category: "internal_url"},
	{name: "Internal Service URL", re: regexp.MustCompile(`https?://(?:internal|staging|dev|test|local|debug)(?:[.:/-]|$)[^"'\s]*`), severity: models.SeverityMedium, category: "internal_url"},
	{name: "IP Address URL", re: regexp.MustCompile(`https?://\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}[^"'\s]*`), severity: models.SeverityMedium, category: "internal_url"},

	// Auth storage patterns
	{name: "localStorage Auth", re: regexp.MustCompile(`localStorage\.setItem\s*\(\s*["'](?:token|auth|session|jwt|access_token|refresh_token|id_token)[^)]*\)`), severity: models.SeverityMedium, category: "auth_storage"},
	{name: "sessionStorage Auth", re: regexp.MustCompile(`sessionStorage\.setItem\s*\(\s*["'](?:token|auth|session|jwt)[^)]*\)`), severity: models.SeverityMedium, category: "auth_storage"},
	{name: "Cookie Assignment", re: regexp.MustCompile(`document\.cookie\s*=\s*["'][^"']*(?:token|session|auth)[^"']*["']`), severity: models.SeverityMedium, category: "auth_storage"},
	{name: "CORS Credentials Include", re: regexp.MustCompile(`credentials\s*:\s*["']include["']`), severity: models.SeverityLow, category: "auth_storage"},

	// Debug artifacts
	{name: "Console Log Sensitive", re: regexp.MustCompile(`console\.log\s*\([^)]*(?:token|password|secret|key|auth|credential|session)[^)]*\)`), severity: models.SeverityMedium, category: "debug"},
	{name: "Debugger Statement", re: regexp.MustCompile(`\bdebugger\s*;`), severity: models.SeverityLow, category: "debug"},
}

// extractSecretsFromJS scans JS/HTML content for secrets and emits them via sink.
func extractSecretsFromJS(ctx context.Context, sink engine.ResultSink, content, sourceURL string) {
	seen := make(map[string]bool)

	for _, sp := range secretPatterns {
		matches := sp.re.FindAllString(content, 10) // cap at 10 per pattern per file
		for _, match := range matches {
			// Dedup by pattern+match value
			key := sp.name + ":" + match
			if seen[key] {
				continue
			}
			seen[key] = true

			// Truncate match for storage (some matches can be very long)
			value := match
			if len(value) > 200 {
				value = value[:200] + "..."
			}

			ctxStr := fmt.Sprintf("[%s] Found in %s", sp.category, sourceURL)
			sink.AddSecret(ctx, &models.Secret{
				SourceURL:  sourceURL,
				SecretType: sp.name,
				Value:      value,
				Context:    &ctxStr,
				Source:     "static-analysis",
				Severity:   sp.severity,
			})
		}
	}
}
