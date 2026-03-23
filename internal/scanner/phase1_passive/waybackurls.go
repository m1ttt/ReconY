package phase1

import (
	"context"
	"fmt"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// WaybackURLsRunner fetches historical URLs from the Wayback Machine.
type WaybackURLsRunner struct{}

func (w *WaybackURLsRunner) Name() string         { return "waybackurls" }
func (w *WaybackURLsRunner) Phase() engine.PhaseID { return engine.PhasePassive }
func (w *WaybackURLsRunner) Check() error          { return tools.CheckBinary("waybackurls") }

func (w *WaybackURLsRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	result, err := tools.RunToolWithProxy(ctx, "waybackurls", []string{domain}, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			url := strings.TrimSpace(line)
			if url != "" {
				sink.AddHistoricalURL(ctx, &models.HistoricalURL{
					URL:    url,
					Source: "waybackurls",
				})
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running waybackurls: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("waybackurls exited with code %d", result.ExitCode)
	}

	return nil
}
