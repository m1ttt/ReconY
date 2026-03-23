package phase4

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// HTTPXRunner uses httpx for tech detection and probing.
type HTTPXRunner struct{}

func (h *HTTPXRunner) Name() string         { return "httpx" }
func (h *HTTPXRunner) Phase() engine.PhaseID { return engine.PhaseFingerprint }
func (h *HTTPXRunner) Check() error          { return tools.CheckBinary("httpx") }

func (h *HTTPXRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
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

	tmpDir := os.TempDir()
	inputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-httpx-%s.txt", input.ScanJobID))
	defer os.Remove(inputFile)
	os.WriteFile(inputFile, []byte(strings.Join(targets, "\n")), 0644)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Probing %d targets with httpx", len(targets)))

	tc := input.Config.GetToolConfig("httpx")
	args := []string{
		"-l", inputFile,
		"-json",
		"-tech-detect",
		"-status-code",
		"-title",
		"-server",
		"-content-type",
		"-follow-redirects",
		"-silent",
	}
	if tc.Threads != nil {
		args = append(args, "-threads", fmt.Sprintf("%d", *tc.Threads))
	}
	args = append(args, tc.ExtraArgs...)

	result, err := tools.RunToolWithProxy(ctx, "httpx", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry httpxResult
			if json.Unmarshal([]byte(line), &entry) != nil {
				return
			}
			h.processResult(ctx, &entry, input, sink)
		}
	})
	if err != nil {
		return fmt.Errorf("running httpx: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("httpx exited with code %d", result.ExitCode)
	}

	return nil
}

type httpxResult struct {
	URL         string            `json:"url"`
	Input       string            `json:"input"`
	StatusCode  int               `json:"status_code"`
	Title       string            `json:"title"`
	Tech        []string          `json:"tech"`
	Server      string            `json:"webserver"`
	ContentType string            `json:"content_type"`
	Host        string            `json:"host"`
	Port        string            `json:"port"`
	Scheme      string            `json:"scheme"`
	Headers     map[string]string `json:"header"`
}

func (h *HTTPXRunner) processResult(ctx context.Context, entry *httpxResult, input *engine.PhaseInput, sink engine.ResultSink) {
	// Mark host as alive in subdomains table when httpx returns a result.
	aliveHost := httpxHostname(entry)
	if aliveHost != "" {
		_ = sink.AddSubdomain(ctx, &models.Subdomain{
			Hostname: aliveHost,
			IsAlive:  true,
			Source:   "httpx",
		})
	}

	// Find matching subdomain
	var subdomainID *string
	for _, sub := range input.Subdomains {
		if sub.Hostname == aliveHost || sub.Hostname == entry.Host || sub.Hostname == entry.Input {
			subdomainID = &sub.ID
			break
		}
	}

	// Add technologies
	for _, tech := range entry.Tech {
		parts := strings.SplitN(tech, " ", 2)
		name := parts[0]
		var version *string
		if len(parts) > 1 {
			v := parts[1]
			version = &v
		}

		category := categorizeTech(name)
		sink.AddTechnology(ctx, &models.Technology{
			SubdomainID: subdomainID,
			URL:         entry.URL,
			Name:        name,
			Version:     version,
			Category:    &category,
			Confidence:  90,
		})
	}

	// Server header as technology
	if entry.Server != "" {
		cat := "server"
		sink.AddTechnology(ctx, &models.Technology{
			SubdomainID: subdomainID,
			URL:         entry.URL,
			Name:        entry.Server,
			Category:    &cat,
			Confidence:  95,
		})
	}
}

func httpxHostname(entry *httpxResult) string {
	if entry == nil {
		return ""
	}
	host := strings.TrimSpace(entry.Input)
	if host == "" && entry.URL != "" {
		if u, err := url.Parse(entry.URL); err == nil {
			host = strings.TrimSpace(u.Hostname())
		}
	}
	if host == "" {
		host = strings.TrimSpace(entry.Host)
	}
	host = strings.TrimSuffix(host, ".")
	return host
}

func categorizeTech(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "wordpress") || strings.Contains(lower, "drupal") ||
		strings.Contains(lower, "joomla") || strings.Contains(lower, "magento"):
		return "cms"
	case strings.Contains(lower, "react") || strings.Contains(lower, "vue") ||
		strings.Contains(lower, "angular") || strings.Contains(lower, "next") ||
		strings.Contains(lower, "nuxt") || strings.Contains(lower, "svelte"):
		return "framework"
	case strings.Contains(lower, "nginx") || strings.Contains(lower, "apache") ||
		strings.Contains(lower, "iis") || strings.Contains(lower, "caddy"):
		return "server"
	case strings.Contains(lower, "cloudflare") || strings.Contains(lower, "akamai") ||
		strings.Contains(lower, "fastly"):
		return "cdn"
	case strings.Contains(lower, "php") || strings.Contains(lower, "python") ||
		strings.Contains(lower, "java") || strings.Contains(lower, "ruby") ||
		strings.Contains(lower, "node") || strings.Contains(lower, "go"):
		return "language"
	case strings.Contains(lower, "mysql") || strings.Contains(lower, "postgres") ||
		strings.Contains(lower, "mongo") || strings.Contains(lower, "redis"):
		return "database"
	default:
		return "other"
	}
}
