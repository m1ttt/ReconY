package phase5

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/tools"
)

// CMSeekRunner detects CMS type and version.
type CMSeekRunner struct{}

func (c *CMSeekRunner) Name() string         { return "cmseek" }
func (c *CMSeekRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (c *CMSeekRunner) Check() error          { return tools.CheckBinary("cmseek") }

func (c *CMSeekRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	// Only run for classic/unknown sites
	var targets []string
	for _, cl := range input.Classifications {
		if cl.SiteType == models.SiteTypeClassic || cl.SiteType == models.SiteTypeUnknown {
			targets = append(targets, cl.URL)
		}
	}
	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			if sub.IsAlive {
				targets = append(targets, "https://"+sub.Hostname)
			}
		}
	}
	if len(targets) == 0 {
		return nil
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("CMS detection on %d targets", len(targets)))

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		result, err := tools.RunToolWithProxy(ctx, "cmseek", []string{"-u", target, "--batch"}, input.ProxyURL, func(stream, line string) {
			sink.LogLine(ctx, stream, line)
		})
		if err != nil || result.ExitCode != 0 {
			continue
		}

		// CMSeeK outputs results to a JSON file
		c.parseResults(ctx, target, input, sink)
	}

	return nil
}

func (c *CMSeekRunner) parseResults(ctx context.Context, target string, input *engine.PhaseInput, sink engine.ResultSink) {
	// CMSeek writes to ~/.cmseek/Result/<domain>/cms.json
	home, _ := os.UserHomeDir()
	domain := extractDomain(target)
	resultFile := filepath.Join(home, ".cmseek", "Result", domain, "cms.json")

	data, err := os.ReadFile(resultFile)
	if err != nil {
		return
	}

	var result struct {
		CMSName    string `json:"cms_name"`
		CMSVersion string `json:"cms_version"`
		CMSURL     string `json:"cms_url"`
	}
	if json.Unmarshal(data, &result) != nil || result.CMSName == "" {
		return
	}

	var subdomainID *string
	for _, sub := range input.Subdomains {
		if strings.Contains(target, sub.Hostname) {
			subdomainID = &sub.ID
			break
		}
	}

	cat := "cms"
	var version *string
	if result.CMSVersion != "" {
		version = &result.CMSVersion
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: CMS detected: %s %s", target, result.CMSName, stringVal(version)))

	sink.AddTechnology(ctx, &models.Technology{
		SubdomainID: subdomainID,
		URL:         target,
		Name:        result.CMSName,
		Version:     version,
		Category:    &cat,
		Confidence:  95,
	})
}

func extractDomain(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	parts := strings.SplitN(u, "/", 2)
	return parts[0]
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
