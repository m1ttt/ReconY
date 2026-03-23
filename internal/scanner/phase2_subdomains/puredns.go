package phase2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// PureDNSRunner resolves and validates discovered subdomains using puredns.
type PureDNSRunner struct{}

func (p *PureDNSRunner) Name() string         { return "puredns" }
func (p *PureDNSRunner) Phase() engine.PhaseID { return engine.PhaseSubdomains }
func (p *PureDNSRunner) Check() error          { return tools.CheckBinary("puredns") }

func (p *PureDNSRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	// Collect all subdomains discovered so far (from subfinder, crtsh, etc.)
	if len(input.Subdomains) == 0 {
		sink.LogLine(ctx, "stdout", "No subdomains to resolve yet, skipping puredns")
		return nil
	}

	// Write subdomains to a temp file
	tmpDir := os.TempDir()
	inputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-puredns-%s.txt", input.ScanJobID))
	defer os.Remove(inputFile)

	var lines []string
	for _, sub := range input.Subdomains {
		lines = append(lines, sub.Hostname)
	}

	if err := os.WriteFile(inputFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Resolving %d subdomains with puredns...", len(lines)))

	args := []string{"resolve", inputFile, "--quiet"}

	// Add resolvers file if configured
	tc := input.Config.GetToolConfig("puredns")
	if resolvers, ok := tc.Options["resolvers"]; ok {
		if resolversStr, ok := resolvers.(string); ok && resolversStr != "" {
			args = append(args, "-r", resolversStr)
		}
	}

	alive := make(map[string]bool)
	result, err := tools.RunToolWithProxy(ctx, "puredns", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			hostname := strings.TrimSpace(line)
			if hostname != "" {
				alive[hostname] = true
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running puredns: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("puredns exited with code %d", result.ExitCode)
	}

	// Update subdomains with alive status and resolved IPs
	for _, sub := range input.Subdomains {
		if alive[sub.Hostname] {
			// Resolve IPs
			ips := resolveIPs(sub.Hostname)
			var ipStr *string
			if len(ips) > 0 {
				ipJSON, _ := json.Marshal(ips)
				s := string(ipJSON)
				ipStr = &s
			}

			sink.AddSubdomain(ctx, &models.Subdomain{
				Hostname:    sub.Hostname,
				IPAddresses: ipStr,
				IsAlive:     true,
				Source:      sub.Source,
			})
		}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("%d/%d subdomains alive", len(alive), len(lines)))
	return nil
}

func resolveIPs(hostname string) []string {
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return nil
	}
	return addrs
}
