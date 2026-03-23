package phase1

import (
	"context"
	"fmt"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// GAURunner fetches URLs from AlienVault OTX, Wayback, and Common Crawl.
type GAURunner struct{}

func (g *GAURunner) Name() string         { return "gau" }
func (g *GAURunner) Phase() engine.PhaseID { return engine.PhasePassive }
func (g *GAURunner) Check() error          { return tools.CheckBinary("gau") }

func (g *GAURunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	result, err := tools.RunToolWithProxy(ctx, "gau", []string{"--subs", domain}, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			url := strings.TrimSpace(line)
			if url != "" {
				sink.AddHistoricalURL(ctx, &models.HistoricalURL{
					URL:    url,
					Source: "gau",
				})
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running gau: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("gau exited with code %d", result.ExitCode)
	}

	return nil
}
