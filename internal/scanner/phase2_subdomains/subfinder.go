package phase2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// SubfinderRunner discovers subdomains using subfinder.
type SubfinderRunner struct{}

func (s *SubfinderRunner) Name() string         { return "subfinder" }
func (s *SubfinderRunner) Phase() engine.PhaseID { return engine.PhaseSubdomains }
func (s *SubfinderRunner) Check() error          { return tools.CheckBinary("subfinder") }

func (s *SubfinderRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	args := []string{"-d", domain, "-silent", "-json"}

	// Apply thread config
	tc := input.Config.GetToolConfig("subfinder")
	if tc.Threads != nil {
		args = append(args, "-t", strconv.Itoa(*tc.Threads))
	}
	args = append(args, tc.ExtraArgs...)

	result, err := tools.RunToolWithProxy(ctx, "subfinder", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry struct {
				Host   string `json:"host"`
				Source string `json:"source"`
			}
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Host != "" {
				sink.AddSubdomain(ctx, &models.Subdomain{
					Hostname: entry.Host,
					Source:   "subfinder",
				})
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running subfinder: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("subfinder exited with code %d", result.ExitCode)
	}

	return nil
}
