package phase2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// CrtshRunner discovers subdomains via crt.sh certificate transparency logs.
type CrtshRunner struct{}

func (c *CrtshRunner) Name() string         { return "crtsh" }
func (c *CrtshRunner) Phase() engine.PhaseID { return engine.PhaseSubdomains }
func (c *CrtshRunner) Check() error          { return nil } // No binary needed, uses HTTP API

func (c *CrtshRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	domain := input.Workspace.Domain

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Querying crt.sh for *.%s", domain))

	client := httpkit.NewClient(input.Config)
	subdomains, err := c.queryCrtsh(ctx, domain, client)
	if err != nil {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("crt.sh API failed: %v, trying alternative...", err))
		// Fallback: try with crtsher if available
		subdomains, err = c.queryCrtsher(ctx, domain, sink)
		if err != nil {
			return fmt.Errorf("crt.sh query failed: %w", err)
		}
	}

	seen := make(map[string]bool)
	for _, hostname := range subdomains {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		hostname = strings.TrimPrefix(hostname, "*.")
		if hostname == "" || seen[hostname] {
			continue
		}
		seen[hostname] = true

		// Only include subdomains of the target domain
		if !strings.HasSuffix(hostname, "."+domain) && hostname != domain {
			continue
		}

		sink.LogLine(ctx, "stdout", hostname)
		if err := sink.AddSubdomain(ctx, &models.Subdomain{
			Hostname: hostname,
			Source:   "crtsh",
		}); err != nil {
			return err
		}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Found %d unique subdomains via crt.sh", len(seen)))
	return nil
}

func (c *CrtshRunner) queryCrtsh(ctx context.Context, domain string, client *httpkit.Client) ([]string, error) {
	url := fmt.Sprintf("https://crt.sh/?q=%%.%s&output=json", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "reconx/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries []struct {
		CommonName string `json:"common_name"`
		NameValue  string `json:"name_value"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing crt.sh response: %w", err)
	}

	var subdomains []string
	for _, entry := range entries {
		// name_value can contain multiple names separated by newlines
		for _, name := range strings.Split(entry.NameValue, "\n") {
			subdomains = append(subdomains, name)
		}
		subdomains = append(subdomains, entry.CommonName)
	}

	return subdomains, nil
}

func (c *CrtshRunner) queryCrtsher(ctx context.Context, domain string, sink engine.ResultSink) ([]string, error) {
	// crtsher is a CLI tool that handles large crt.sh responses better
	var subdomains []string

	_, err := tools.RunToolWithProxy(ctx, "crtsher", []string{"-d", domain}, "", func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			hostname := strings.TrimSpace(line)
			if hostname != "" {
				subdomains = append(subdomains, hostname)
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return subdomains, nil
}
