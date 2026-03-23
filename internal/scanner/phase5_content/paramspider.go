package phase5

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// ParamSpiderRunner discovers URL parameters.
type ParamSpiderRunner struct{}

func (p *ParamSpiderRunner) Name() string         { return "paramspider" }
func (p *ParamSpiderRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (p *ParamSpiderRunner) Check() error          { return tools.CheckBinary("paramspider") }

func (p *ParamSpiderRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	// Only run for SSR/Classic sites (not SPAs)
	var targets []string
	for _, c := range input.Classifications {
		if c.SiteType == models.SiteTypeSSR || c.SiteType == models.SiteTypeClassic || c.SiteType == models.SiteTypeUnknown {
			targets = append(targets, c.URL)
		}
	}

	if len(targets) == 0 {
		// Fallback to all alive subdomains
		for _, sub := range input.Subdomains {
			if sub.IsAlive {
				targets = append(targets, sub.Hostname)
			}
		}
	}
	if len(targets) == 0 {
		targets = []string{input.Workspace.Domain}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Discovering parameters on %d targets", len(targets)))

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Extract domain from URL
		domain := target
		if u, err := url.Parse(target); err == nil && u.Host != "" {
			domain = u.Host
		}

		result, err := tools.RunToolWithProxy(ctx, "paramspider", []string{"-d", domain, "--quiet"}, input.ProxyURL, func(stream, line string) {
			sink.LogLine(ctx, stream, line)
			if stream == "stdout" {
				paramURL := strings.TrimSpace(line)
				if paramURL == "" || strings.HasPrefix(paramURL, "[") {
					return
				}

				// Extract parameters from URL
				if u, err := url.Parse(paramURL); err == nil {
					for paramName := range u.Query() {
						sink.AddParameter(ctx, &models.Parameter{
							URL:       paramURL,
							Name:      paramName,
							ParamType: "query",
							Source:    "paramspider",
						})
					}
				}

				sink.AddDiscoveredURL(ctx, &models.DiscoveredURL{
					URL:    paramURL,
					Source: "paramspider",
				})
			}
		})
		if err != nil || result.ExitCode != 0 {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("paramspider failed for %s", domain))
		}
	}

	return nil
}
