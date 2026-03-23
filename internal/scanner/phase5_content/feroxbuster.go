package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"reconx/internal/config"
	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
	"reconx/internal/tools"
	"reconx/internal/wordlist"
)

// FeroxbusterRunner performs recursive directory/content discovery.
type FeroxbusterRunner struct {
	Selector *wordlist.Selector
	Resolver *wordlist.Resolver
}

func (f *FeroxbusterRunner) Name() string         { return "feroxbuster" }
func (f *FeroxbusterRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (f *FeroxbusterRunner) Check() error          { return tools.CheckBinary("feroxbuster") }

func (f *FeroxbusterRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	tc := input.Config.GetToolConfig("feroxbuster")

	// Build wordlist (same logic as ffuf)
	var wordlists []string
	tier := wordlist.TierStandard
	wordlists = append(wordlists, f.Selector.WebWordlist(tier))

	if f.Selector != nil {
		var techNames []string
		for _, t := range input.Technologies {
			techNames = append(techNames, t.Name)
		}
		techWLs := f.Selector.ForTechnologies(techNames)
		wordlists = append(wordlists, techWLs...)

		for _, c := range input.Classifications {
			siteWLs := f.Selector.ForSiteType(string(c.SiteType), tier)
			wordlists = append(wordlists, siteWLs...)
		}
	}

	if tc.WordlistOverrides != nil {
		if wl, ok := tc.WordlistOverrides["web"]; ok {
			wordlists = []string{wl}
		}
	}

	if len(wordlists) == 0 {
		sink.LogLine(ctx, "stderr", "No wordlists available for feroxbuster")
		return nil
	}

	// Merge wordlists
	tmpDir := os.TempDir()
	mergedFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-ferox-wl-%s.txt", input.ScanJobID))
	defer os.Remove(mergedFile)

	seen := make(map[string]bool)
	var mergedLines []string
	for _, wl := range wordlists {
		resolvedPath := wl
		if f.Resolver != nil {
			if resolved, err := f.Resolver.Resolve(wl); err == nil {
				resolvedPath = resolved
			}
		}
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Cannot read wordlist %s: %v", wl, err))
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				mergedLines = append(mergedLines, line)
			}
		}
	}
	os.WriteFile(mergedFile, []byte(strings.Join(mergedLines, "\n")), 0644)

	// Config options
	depth := getFeroxIntOption(tc, "depth", 4)
	// feroxbuster: -s and -C are mutually exclusive. Use -s (whitelist) by default.
	statusCodes := getFeroxStringOption(tc, "status_codes", "200,201,301,302,307,308,401,403,405")
	extractLinks := getFeroxBoolOption(tc, "extract_links", true)
	autoTune := getFeroxBoolOption(tc, "auto_tune", true)
	collectExtensions := getFeroxBoolOption(tc, "collect_extensions", true)
	collectBackups := getFeroxBoolOption(tc, "collect_backups", true)

	// Build base targets
	baseTargets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)
	aliveCount := 0
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			aliveCount++
		}
	}
	if len(input.Classifications) == 0 && aliveCount == 0 {
		sink.LogLine(ctx, "stderr", "No pre-verified live HTTP targets from httpx/classify; continuing with fallback targets (this does not mean the site is down)")
	}

	// Seed with directories discovered by previous tools (ffuf, katana, etc.)
	// Extract unique directory paths from DiscoveredURLs to use as extra scan roots.
	seedDirs := extractSeedDirectories(baseTargets, input.DiscoveredURLs)
	if len(seedDirs) > 0 {
		sink.LogLine(ctx, "stdout", fmt.Sprintf("Seeding with %d directories from previous scans", len(seedDirs)))
	}

	// Merge: base targets + seed directories
	targets := append(baseTargets, seedDirs...)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Recursive scan on %d targets with %d words, depth=%d", len(targets), len(mergedLines), depth))

	totalFound := 0
	for i, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		sink.LogLine(ctx, "stdout", fmt.Sprintf("feroxbuster target: %s", target))

		outputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-ferox-out-%s-%d.json", input.ScanJobID, i))

		args := []string{
			"-u", target,
			"-w", mergedFile,
			"-o", outputFile,
			"--json",
			"-d", fmt.Sprintf("%d", depth),
			"-s", statusCodes,
			"-q", // quiet: hide progress bars/banner, still write JSON to -o file
		}

		if extractLinks {
			args = append(args, "-e") // extract links from response bodies
		}
		if autoTune {
			args = append(args, "--auto-tune")
		}
		if collectExtensions {
			args = append(args, "--collect-extensions")
		}
		if collectBackups {
			args = append(args, "-B") // collect backup extensions (.bak, .old, etc.)
		}

		// Threads
		if tc.Threads != nil {
			args = append(args, "-t", fmt.Sprintf("%d", *tc.Threads))
		}
		// Rate limit
		if tc.RateLimit != nil && *tc.RateLimit > 0 {
			args = append(args, "-L", fmt.Sprintf("%d", *tc.RateLimit))
		}

		// Don't follow redirects to external domains
		args = append(args, "--dont-scan", ".*\\.google\\.com.*,.*\\.facebook\\.com.*,.*\\.amazonaws\\.com.*")

		// Filter by response size if configured
		if fs := getFeroxStringOption(tc, "filter_size", ""); fs != "" {
			args = append(args, "-S", fs)
		}
		if fw := getFeroxStringOption(tc, "filter_words", ""); fw != "" {
			args = append(args, "-W", fw)
		}
		if fl := getFeroxStringOption(tc, "filter_lines", ""); fl != "" {
			args = append(args, "-N", fl)
		}

		// Extra args from config
		args = append(args, tc.ExtraArgs...)

		// Auth headers — CLIHeaders() returns ["-H", "Key: Value", "-H", "Key: Value"]
		for _, sess := range input.AuthSessions {
			args = append(args, sess.CLIHeaders()...)
		}

		result, err := tools.RunToolWithProxy(ctx, "feroxbuster", args, input.ProxyURL, func(stream, line string) {
			sink.LogLine(ctx, stream, line)
		})
		persistCtx := ctx
		if persistCtx.Err() != nil {
			persistCtx = context.Background()
		}

		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("feroxbuster error on %s: %v", target, err))
			// Attempt to recover partial findings if output file exists.
			found := parseFeroxOutput(persistCtx, outputFile, target, sink)
			totalFound += found
			if found == 0 {
				sink.LogLine(persistCtx, "stderr", fmt.Sprintf("feroxbuster target produced no matches before error: %s", target))
			} else {
				sink.LogLine(persistCtx, "stdout", fmt.Sprintf("feroxbuster target found %d URL(s) before error: %s", found, target))
			}
			os.Remove(outputFile)
			continue
		}
		if result.ExitCode != 0 && result.ExitCode != 1 {
			// feroxbuster returns 1 when some URLs filtered
			sink.LogLine(ctx, "stderr", fmt.Sprintf("feroxbuster exited %d on %s", result.ExitCode, target))
		}

		// Parse results even for non-zero exits to preserve partial findings.
		found := parseFeroxOutput(persistCtx, outputFile, target, sink)
		totalFound += found
		if found == 0 {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("feroxbuster target had 0 matches: %s (host may be unreachable, heavily filtered, or no matching paths)", target))
		} else {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("feroxbuster target found %d URL(s): %s", found, target))
		}
		os.Remove(outputFile)
	}
	if totalFound == 0 {
		sink.LogLine(ctx, "stderr", "feroxbuster finished with 0 total matches across all targets")
	}

	return nil
}

type feroxResult struct {
	URL           string            `json:"url"`
	Status        int               `json:"status"`
	ContentLength int               `json:"content_length"`
	WordCount     int               `json:"word_count"`
	LineCount     int               `json:"line_count"`
	ContentType   string            `json:"content_type"`
	Type          string            `json:"type"`
	OriginalURL   string            `json:"original_url"`
	RedirectURL   string            `json:"redirect_url"`
	Path          string            `json:"path"`
	Headers       map[string]string `json:"headers"`
}

func parseFeroxOutput(ctx context.Context, outputFile, baseURL string, sink engine.ResultSink) int {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return 0
	}

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var r feroxResult
		if json.Unmarshal([]byte(line), &r) != nil {
			continue
		}

		// Only process response entries (not stats/progress)
		if r.Type != "" && r.Type != "response" {
			continue
		}

		// Skip if no URL
		if r.URL == "" {
			continue
		}

		statusCode := r.Status
		contentLength := r.ContentLength
		var ct *string
		// content_type comes from headers map in feroxbuster JSON
		if r.ContentType != "" {
			ct = &r.ContentType
		} else if h, ok := r.Headers["content-type"]; ok && h != "" {
			ct = &h
		}

		// Track redirect location if present
		var redirectLoc *string
		if r.RedirectURL != "" {
			redirectLoc = &r.RedirectURL
		}

		_ = sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
			URL:              r.URL,
			StatusCode:       &statusCode,
			ContentType:      ct,
			ContentLength:    &contentLength,
			RedirectLocation: redirectLoc,
			Source:           "feroxbuster",
		})
		count++
	}

	return count
}

func getFeroxStringOption(tc config.ToolConfig, key, fallback string) string {
	return getFFUFStringOption(tc, key, fallback) // reuse same logic
}

func getFeroxBoolOption(tc config.ToolConfig, key string, fallback bool) bool {
	return getFFUFBoolOption(tc, key, fallback) // reuse same logic
}

func getFeroxIntOption(tc config.ToolConfig, key string, fallback int) int {
	if tc.Options == nil {
		return fallback
	}
	v, ok := tc.Options[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

// extractSeedDirectories derives directory URLs from previously discovered URLs.
// For example, if ffuf found https://example.com/admin/login (200), we extract
// https://example.com/admin/ as a directory to recursively scan.
// Only returns directories not already covered by baseTargets.
func extractSeedDirectories(baseTargets []string, discoveredURLs []models.DiscoveredURL) []string {
	baseSet := make(map[string]bool)
	for _, t := range baseTargets {
		baseSet[strings.TrimRight(t, "/")] = true
	}

	dirSet := make(map[string]bool)
	var dirs []string

	for _, u := range discoveredURLs {
		// Only seed from successful responses (not redirects/errors)
		if u.StatusCode == nil || *u.StatusCode >= 400 {
			continue
		}

		parsed, err := url.Parse(u.URL)
		if err != nil || parsed.Host == "" {
			continue
		}

		// Walk up the path to extract parent directories
		// e.g. /admin/api/v2/users → /admin/api/v2/, /admin/api/, /admin/
		dir := path.Dir(parsed.Path)
		for dir != "" && dir != "/" && dir != "." {
			dirURL := fmt.Sprintf("%s://%s%s/", parsed.Scheme, parsed.Host, dir)
			norm := strings.TrimRight(dirURL, "/")

			// Skip if it's a base target or already seen
			if !baseSet[norm] && !dirSet[norm] {
				dirSet[norm] = true
				dirs = append(dirs, dirURL)
			}
			dir = path.Dir(dir)
		}
	}

	// Cap to prevent explosion on large result sets
	if len(dirs) > 100 {
		dirs = dirs[:100]
	}

	return dirs
}
