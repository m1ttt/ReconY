package phase7

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
	"reconx/internal/tools"
)

// NucleiRunner runs nuclei vulnerability scanner.
type NucleiRunner struct{}

func (n *NucleiRunner) Name() string         { return "nuclei" }
func (n *NucleiRunner) Phase() engine.PhaseID { return engine.PhaseVulns }
func (n *NucleiRunner) Check() error          { return tools.CheckBinary("nuclei") }

func (n *NucleiRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	tmpDir := os.TempDir()
	inputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-nuclei-%s.txt", input.ScanJobID))
	defer os.Remove(inputFile)
	os.WriteFile(inputFile, []byte(strings.Join(targets, "\n")), 0644)

	tc := input.Config.GetToolConfig("nuclei")
	args := []string{"-l", inputFile, "-jsonl", "-silent"}

	// Severity filter
	if opts := tc.Options; opts != nil {
		if sev, ok := opts["severity"].([]any); ok {
			var sevStrs []string
			for _, s := range sev {
				if str, ok := s.(string); ok {
					sevStrs = append(sevStrs, str)
				}
			}
			if len(sevStrs) > 0 {
				args = append(args, "-severity", strings.Join(sevStrs, ","))
			}
		}
		// Tags
		if tags, ok := opts["tags"].([]any); ok {
			var tagStrs []string
			for _, t := range tags {
				if str, ok := t.(string); ok {
					tagStrs = append(tagStrs, str)
				}
			}
			if len(tagStrs) > 0 {
				args = append(args, "-tags", strings.Join(tagStrs, ","))
			}
		}
		// Exclude tags
		if etags, ok := opts["exclude_tags"].([]any); ok {
			var etagStrs []string
			for _, t := range etags {
				if str, ok := t.(string); ok {
					etagStrs = append(etagStrs, str)
				}
			}
			if len(etagStrs) > 0 {
				args = append(args, "-exclude-tags", strings.Join(etagStrs, ","))
			}
		}
	}

	// Rate limit
	if tc.RateLimit != nil && *tc.RateLimit > 0 {
		args = append(args, "-rate-limit", fmt.Sprintf("%d", *tc.RateLimit))
	}

	// Custom templates path
	if opts := tc.Options; opts != nil {
		if tplPath, ok := opts["templates_path"].(string); ok && tplPath != "" {
			args = append(args, "-t", tplPath)
		}
	}

	args = append(args, tc.ExtraArgs...)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Running nuclei on %d targets", len(targets)))

	result, err := tools.RunToolWithProxy(ctx, "nuclei", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
		if stream == "stdout" {
			var entry nucleiResult
			if json.Unmarshal([]byte(line), &entry) == nil && entry.TemplateID != "" {
				n.processResult(ctx, &entry, input, sink)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("running nuclei: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("nuclei exited with code %d", result.ExitCode)
	}

	return nil
}

type nucleiResult struct {
	TemplateID string `json:"template-id"`
	Info       struct {
		Name        string   `json:"name"`
		Severity    string   `json:"severity"`
		Description string   `json:"description"`
		Reference   []string `json:"reference"`
		Tags        []string `json:"tags"`
	} `json:"info"`
	MatcherName string `json:"matcher-name"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Matched     string `json:"matched-at"`
	ExtractedResults []string `json:"extracted-results"`
	CurlCommand string `json:"curl-command"`
}

func (n *NucleiRunner) processResult(ctx context.Context, entry *nucleiResult, input *engine.PhaseInput, sink engine.ResultSink) {
	var subdomainID *string
	for _, sub := range input.Subdomains {
		if strings.Contains(entry.Host, sub.Hostname) {
			subdomainID = &sub.ID
			break
		}
	}

	severity := models.Severity(strings.ToLower(entry.Info.Severity))
	if severity == "" {
		severity = models.SeverityInfo
	}

	var matchedAt, description, reference, curlCmd, extracted *string
	if entry.Matched != "" {
		matchedAt = &entry.Matched
	}
	if entry.Info.Description != "" {
		description = &entry.Info.Description
	}
	if len(entry.Info.Reference) > 0 {
		refJSON, _ := json.Marshal(entry.Info.Reference)
		refStr := string(refJSON)
		reference = &refStr
	}
	if entry.CurlCommand != "" {
		curlCmd = &entry.CurlCommand
	}
	if len(entry.ExtractedResults) > 0 {
		extJSON, _ := json.Marshal(entry.ExtractedResults)
		extStr := string(extJSON)
		extracted = &extStr
	}

	url := entry.Host
	if entry.Matched != "" {
		url = entry.Matched
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("[%s] %s: %s @ %s",
		entry.Info.Severity, entry.TemplateID, entry.Info.Name, url))

	sink.AddVulnerability(ctx, &models.Vulnerability{
		SubdomainID: subdomainID,
		TemplateID:  entry.TemplateID,
		Name:        entry.Info.Name,
		Severity:    severity,
		URL:         url,
		MatchedAt:   matchedAt,
		Description: description,
		Reference:   reference,
		CurlCommand: curlCmd,
		Extracted:   extracted,
	})
}
