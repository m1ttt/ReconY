package phase2

import (
	"context"
	"fmt"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// AmassRunner discovers subdomains using OWASP Amass.
type AmassRunner struct{}

func (a *AmassRunner) Name() string         { return "amass" }
func (a *AmassRunner) Phase() engine.PhaseID { return engine.PhaseSubdomains }
func (a *AmassRunner) Check() error          { return tools.CheckBinary("amass") }

func (a *AmassRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	args := []string{"enum", "-passive", "-d", domain}

	tc := input.Config.GetToolConfig("amass")
	if tc.Timeout != "" {
		args = append(args, "-timeout", strings.TrimSuffix(tc.Timeout, "m"))
	}
	args = append(args, tc.ExtraArgs...)

	result, err := tools.RunToolWithProxy(ctx, "amass", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			host := strings.TrimSpace(line)
			if host != "" && !strings.HasPrefix(host, "//") && strings.Contains(host, ".") {
				sink.AddSubdomain(ctx, &models.Subdomain{
					Hostname: host,
					Source:   "amass",
				})
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running amass: %w", err)
	}
	_ = result
	// amass can exit non-zero for various non-fatal reasons, don't fail
	return nil
}

