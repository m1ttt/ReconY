package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"reconx/internal/config"
	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
	"reconx/internal/tools"
	"reconx/internal/wordlist"
)

// FFUFRunner performs directory/file fuzzing.
type FFUFRunner struct {
	Selector *wordlist.Selector
	Resolver *wordlist.Resolver
}

func (f *FFUFRunner) Name() string         { return "ffuf" }
func (f *FFUFRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (f *FFUFRunner) Check() error          { return tools.CheckBinary("ffuf") }

func (f *FFUFRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	tc := input.Config.GetToolConfig("ffuf")

	// Determine wordlists based on detected tech
	var wordlists []string

	// Base wordlist from tier
	tier := wordlist.TierStandard
	wordlists = append(wordlists, f.Selector.WebWordlist(tier))

	// Smart tech-specific wordlists
	if f.Selector != nil {
		var techNames []string
		for _, t := range input.Technologies {
			techNames = append(techNames, t.Name)
		}
		techWLs := f.Selector.ForTechnologies(techNames)
		wordlists = append(wordlists, techWLs...)

		// API-specific if site classified as API
		for _, c := range input.Classifications {
			siteWLs := f.Selector.ForSiteType(string(c.SiteType), tier)
			wordlists = append(wordlists, siteWLs...)
		}
	}

	// Override from tool config
	if tc.WordlistOverrides != nil {
		if wl, ok := tc.WordlistOverrides["web"]; ok {
			wordlists = []string{wl}
		}
	}

	if len(wordlists) == 0 {
		sink.LogLine(ctx, "stderr", "No wordlists available for ffuf")
		return nil
	}

	// Merge wordlists into one temp file
	tmpDir := os.TempDir()
	mergedFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-ffuf-wl-%s.txt", input.ScanJobID))
	defer os.Remove(mergedFile)

	seen := make(map[string]bool)
	var mergedLines []string
	for _, wl := range wordlists {
		// Resolve relative paths against SecLists
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

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Fuzzing with %d words from %d wordlists", len(mergedLines), len(wordlists)))

	// Read match/filter codes from config
	matchCodes := getFFUFStringOption(tc, "match_codes", "200,201,301,302,403")
	filterCodes := getFFUFStringOption(tc, "filter_codes", "404")
	followRedirects := getFFUFBoolOption(tc, "follow_redirects", true)
	autoCalibrate := getFFUFBoolOption(tc, "auto_calibrate", true)
	maxTimeSeconds := 0
	if tc.Timeout != "" {
		if d, err := time.ParseDuration(tc.Timeout); err == nil {
			// End ffuf slightly before the engine deadline so it can flush partial output.
			if d > 30*time.Second {
				maxTimeSeconds = int(d.Seconds()) - 10
			}
		}
	}
	if maxTimeSeconds > 0 {
		sink.LogLine(ctx, "stdout", fmt.Sprintf("ffuf max runtime per target: %ds", maxTimeSeconds))
	}

	// If following redirects, remove 301,302 from match codes (ffuf reports final response)
	if followRedirects {
		matchCodes = removeRedirectCodes(matchCodes)
	}

	// Run ffuf per target
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)
	aliveCount := 0
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			aliveCount++
		}
	}
	if len(input.Classifications) == 0 && aliveCount == 0 {
		sink.LogLine(ctx, "stderr", "No pre-verified live HTTP targets from httpx/classify; continuing with fallback targets (this does not mean the site is down)")
	}
	sink.LogLine(ctx, "stdout", fmt.Sprintf("ffuf scanning %d target(s)", len(targets)))

	outputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-ffuf-out-%s.json", input.ScanJobID))
	defer os.Remove(outputFile)

	totalFound := 0
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		seenURLs := make(map[string]bool)
		sink.LogLine(ctx, "stdout", fmt.Sprintf("ffuf target: %s", target))
		fuzzURL := target + "/FUZZ"
		args := []string{
			"-u", fuzzURL,
			"-w", mergedFile,
			"-o", outputFile,
			"-of", "json",
			"-json",
			"-mc", matchCodes,
			"-fc", filterCodes,
			"-s", // silent
		}

		// Auto-calibrate: ffuf detects repetitive responses and filters them
		if autoCalibrate {
			args = append(args, "-ac")
		}

		// Follow redirects
		if followRedirects {
			args = append(args, "-r")
		}

		// Optional size/words/lines filters
		if fs := getFFUFStringOption(tc, "filter_size", ""); fs != "" {
			args = append(args, "-fs", fs)
		}
		if fw := getFFUFStringOption(tc, "filter_words", ""); fw != "" {
			args = append(args, "-fw", fw)
		}
		if fl := getFFUFStringOption(tc, "filter_lines", ""); fl != "" {
			args = append(args, "-fl", fl)
		}

		if tc.Threads != nil {
			args = append(args, "-t", fmt.Sprintf("%d", *tc.Threads))
		}
		if tc.RateLimit != nil && *tc.RateLimit > 0 {
			args = append(args, "-rate", fmt.Sprintf("%d", *tc.RateLimit))
		}
		if maxTimeSeconds > 0 {
			args = append(args, "-maxtime", strconv.Itoa(maxTimeSeconds))
		}
		args = append(args, tc.ExtraArgs...)

		// Authenticated fuzzing: inject auth headers
		for _, sess := range input.AuthSessions {
			args = append(args, sess.CLIHeaders()...)
		}

		heartbeatDone := make(chan struct{})
		go func(target string) {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-heartbeatDone:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					sink.LogLine(ctx, "stdout", fmt.Sprintf("ffuf still running on %s ...", target))
				}
			}
		}(target)

		result, err := tools.RunToolWithProxy(ctx, "ffuf", args, input.ProxyURL, func(stream, line string) {
			sink.LogLine(ctx, stream, line)

			// Parse newline-delimited JSON records for live result streaming.
			if stream != "stdout" {
				return
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
				return
			}
			var r ffufResult
			if json.Unmarshal([]byte(trimmed), &r) != nil {
				return
			}
			liveCtx := ctx
			if liveCtx.Err() != nil {
				liveCtx = context.Background()
			}
			addFFUFResult(liveCtx, r, target, sink, seenURLs)
		})
		close(heartbeatDone)

		persistCtx := ctx
		if persistCtx.Err() != nil {
			persistCtx = context.Background()
		}

		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("ffuf error on %s: %v", target, err))
			// Even on execution errors, try to parse any partial output file.
			parseFFUFOutputWithSeen(persistCtx, outputFile, target, sink, seenURLs)
			os.Remove(outputFile)
			if len(seenURLs) == 0 {
				sink.LogLine(persistCtx, "stderr", fmt.Sprintf("ffuf target produced no matches before error: %s", target))
			}
			totalFound += len(seenURLs)
			continue
		}
		if result.ExitCode != 0 {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("ffuf exited %d on %s", result.ExitCode, target))
		}

		// Parse ffuf JSON output as fallback/completion pass.
		parseFFUFOutputWithSeen(persistCtx, outputFile, target, sink, seenURLs)
		os.Remove(outputFile)
		if len(seenURLs) == 0 {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("ffuf target had 0 matches: %s (host may be unreachable, heavily filtered, or no matching paths)", target))
		} else {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("ffuf target found %d URL(s): %s", len(seenURLs), target))
		}
		totalFound += len(seenURLs)
	}

	if totalFound == 0 {
		sink.LogLine(ctx, "stderr", "ffuf finished with 0 total matches across all targets")
		if autoCalibrate {
			sink.LogLine(ctx, "stderr", "Hint: ffuf auto-calibrate is enabled; on WAF-protected targets this can hide matches. Try disabling ffuf.options.auto_calibrate")
		}
	}

	return nil
}

func parseFFUFOutput(ctx context.Context, outputFile, baseURL string, sink engine.ResultSink) {
	parseFFUFOutputWithSeen(ctx, outputFile, baseURL, sink, nil)
}

func parseFFUFOutputWithSeen(ctx context.Context, outputFile, baseURL string, sink engine.ResultSink, seen map[string]bool) {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return
	}

	var output struct {
		Results []ffufResult `json:"results"`
	}

	if json.Unmarshal(data, &output) != nil {
		return
	}

	for _, r := range output.Results {
		addFFUFResult(ctx, r, baseURL, sink, seen)
	}
}

func addFFUFResult(ctx context.Context, r ffufResult, baseURL string, sink engine.ResultSink, seen map[string]bool) {
	// Filter garbage redirects: 301/302 to root or empty location with 0 body
	if isGarbageRedirect(r, baseURL) {
		return
	}

	if strings.TrimSpace(r.URL) == "" {
		return
	}
	if seen != nil {
		if seen[r.URL] {
			return
		}
		seen[r.URL] = true
	}

	statusCode := r.Status
	contentLength := r.Length
	var ct *string
	if r.ContentType != "" {
		ct = &r.ContentType
	}
	var redirectLoc *string
	if r.RedirectLocation != "" {
		redirectLoc = &r.RedirectLocation
	}
	_ = sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
		URL:              r.URL,
		StatusCode:       &statusCode,
		ContentType:      ct,
		ContentLength:    &contentLength,
		RedirectLocation: redirectLoc,
		Source:           "ffuf",
	})
}

type ffufResult struct {
	Input struct {
		FUZZ string `json:"FUZZ"`
	} `json:"input"`
	Status           int    `json:"status"`
	Length           int    `json:"length"`
	Words            int    `json:"words"`
	Lines            int    `json:"lines"`
	ContentType      string `json:"content-type"`
	URL              string `json:"url"`
	RedirectLocation string `json:"redirectlocation"`
}

// isGarbageRedirect returns true if the result is a generic redirect to root
// (e.g., 302 → "/" with 0 bytes body — typical catch-all redirect).
func isGarbageRedirect(r ffufResult, baseURL string) bool {
	if r.Status != 301 && r.Status != 302 && r.Status != 303 && r.Status != 307 && r.Status != 308 {
		return false
	}

	loc := strings.TrimSpace(r.RedirectLocation)

	// Empty redirect location with no body = garbage
	if loc == "" && r.Length == 0 {
		return true
	}

	// Redirect to "/" = catch-all
	if loc == "/" {
		return true
	}

	// Redirect to the base URL itself (with or without trailing slash)
	baseNorm := strings.TrimRight(baseURL, "/")
	locNorm := strings.TrimRight(loc, "/")
	if locNorm == baseNorm {
		return true
	}

	// Redirect to just the domain root (https://example.com or https://example.com/)
	if parsed, err := url.Parse(baseURL); err == nil {
		domainRoot := parsed.Scheme + "://" + parsed.Host
		if locNorm == domainRoot {
			return true
		}
	}

	return false
}

// getFFUFStringOption reads a string option from tool config, with fallback.
func getFFUFStringOption(tc config.ToolConfig, key, fallback string) string {
	if tc.Options == nil {
		return fallback
	}
	v, ok := tc.Options[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case string:
		return val
	case []any:
		// Convert []any{200, 201, 301} → "200,201,301"
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getFFUFBoolOption reads a bool option from tool config, with fallback.
func getFFUFBoolOption(tc config.ToolConfig, key string, fallback bool) bool {
	if tc.Options == nil {
		return fallback
	}
	v, ok := tc.Options[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fallback
		}
		return b
	default:
		return fallback
	}
}

// removeRedirectCodes strips 301,302,303,307,308 from a comma-separated code list.
func removeRedirectCodes(codes string) string {
	redirects := map[string]bool{"301": true, "302": true, "303": true, "307": true, "308": true}
	parts := strings.Split(codes, ",")
	var filtered []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !redirects[p] {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return "200"
	}
	return strings.Join(filtered, ",")
}
