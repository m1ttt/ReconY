package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
	"reconx/internal/tools"
)

// KatanaRunner crawls websites using katana (headless for SPAs).
type KatanaRunner struct{}

func (k *KatanaRunner) Name() string         { return "katana" }
func (k *KatanaRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (k *KatanaRunner) Check() error          { return tools.CheckBinary("katana") }

func (k *KatanaRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	tmpDir := os.TempDir()
	inputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-katana-%s.txt", input.ScanJobID))
	defer os.Remove(inputFile)
	os.WriteFile(inputFile, []byte(strings.Join(targets, "\n")), 0644)

	tc := input.Config.GetToolConfig("katana")
	args := []string{"-list", inputFile, "-jsonl", "-silent"}

	// Depth (default 2 to avoid infinite crawling)
	depth := 2
	if opts := tc.Options; opts != nil {
		if v, ok := opts["depth"].(float64); ok {
			depth = int(v)
		}
	}
	args = append(args, "-d", fmt.Sprintf("%d", depth))

	// Crawl duration limit as safety net
	maxDuration := "120" // 2 min default — most sites finish well before this
	if tc.Timeout != "" {
		if d, err := time.ParseDuration(tc.Timeout); err == nil {
			maxDuration = fmt.Sprintf("%d", int(d.Seconds()))
		}
	}
	args = append(args, "-crawl-duration", maxDuration)

	// Rate limit to avoid hammering
	args = append(args, "-rate-limit", "100")

	// Headless for SPAs
	hasSPA := false
	for _, c := range input.Classifications {
		if c.SiteType == models.SiteTypeSPA || c.SiteType == models.SiteTypeHybrid {
			hasSPA = true
			break
		}
	}
	if hasSPA {
		args = append(args, "-headless", "-jc")
		sink.LogLine(ctx, "stdout", "SPA/Hybrid detected — using headless mode with JS crawling")
	}

	// Authenticated crawling: inject auth headers
	for _, sess := range input.AuthSessions {
		args = append(args, sess.CLIHeaders()...)
		sink.LogLine(ctx, "stdout", fmt.Sprintf("Auth session '%s' injected for crawling", sess.Credential.Name))
	}

	args = append(args, tc.ExtraArgs...)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Crawling %d targets (depth=%d, headless=%v)", len(targets), depth, hasSPA))

	result, err := tools.RunToolWithProxy(ctx, "katana", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry struct {
				Request struct {
					URL    string `json:"endpoint"`
					Method string `json:"method"`
				} `json:"request"`
				Response struct {
					StatusCode int    `json:"status_code"`
					Headers    map[string]any `json:"headers"`
				} `json:"response"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Request.URL != "" {
				var statusCode *int
				if entry.Response.StatusCode > 0 {
					sc := entry.Response.StatusCode
					statusCode = &sc
				}
				sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
					URL:        entry.Request.URL,
					StatusCode: statusCode,
					Source:     "katana",
				})
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running katana: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("katana exited with code %d", result.ExitCode)
	}

	return nil
}
