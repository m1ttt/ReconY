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
	"reconx/internal/scanner"
	"reconx/internal/tools"
)

// GoWitnessRunner captures screenshots of web pages.
type GoWitnessRunner struct{}

func (g *GoWitnessRunner) Name() string         { return "gowitness" }
func (g *GoWitnessRunner) Phase() engine.PhaseID { return engine.PhaseContent }
func (g *GoWitnessRunner) Check() error          { return tools.CheckBinary("gowitness") }

func (g *GoWitnessRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	// Write targets
	tmpDir := os.TempDir()
	inputFile := filepath.Join(tmpDir, fmt.Sprintf("reconx-gowitness-%s.txt", input.ScanJobID))
	defer os.Remove(inputFile)
	os.WriteFile(inputFile, []byte(strings.Join(targets, "\n")), 0644)

	// Output directory
	screenshotDir := input.Config.General.ScreenshotsDir
	wsDir := filepath.Join(screenshotDir, input.Workspace.ID)
	os.MkdirAll(wsDir, 0755)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Taking screenshots of %d targets", len(targets)))

	jsonlFile := filepath.Join(wsDir, "gowitness.jsonl")

	args := []string{
		"scan", "file",
		"-f", inputFile,
		"-s", wsDir,
		"--write-jsonl",
		"--write-jsonl-file", jsonlFile,
	}

	// Inject auth headers for authenticated screenshots
	for _, sess := range input.AuthSessions {
		args = append(args, sess.CLIHeaders()...)
	}

	result, err := tools.RunToolWithProxy(ctx, "gowitness", args, input.ProxyURL, func(stream, line string) {
		sink.LogLine(ctx, stream, line)
	})
	if err != nil {
		return fmt.Errorf("running gowitness: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("gowitness exited with code %d", result.ExitCode)
	}

	// Parse JSONL results
	return g.parseJSONL(ctx, jsonlFile, wsDir, input, sink)
}

func (g *GoWitnessRunner) parseJSONL(ctx context.Context, jsonlFile, screenshotDir string, input *engine.PhaseInput, sink engine.ResultSink) error {
	data, err := os.ReadFile(jsonlFile)
	if err != nil {
		sink.LogLine(ctx, "stderr", "gowitness JSONL not found: "+jsonlFile)
		return nil
	}

	linesRead := 0
	screenshotsParsed := 0

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		linesRead++

		// Support both gowitness v2 and v3 field names
		var result struct {
			URL          string `json:"url"`
			FinalURL     string `json:"final_url"`
			ResponseCode int    `json:"response_code"`
			StatusCode   int    `json:"status_code"`
			Title        string `json:"title"`
			FileName     string `json:"file_name"` // gowitness v3
			Filename     string `json:"filename"`  // alternate
			Screenshot   string `json:"screenshot"`
			Failed       bool   `json:"failed"`
		}
		if json.Unmarshal([]byte(line), &result) != nil {
			sink.LogLine(ctx, "stderr", "gowitness: failed to parse JSONL line")
			continue
		}

		if result.Failed {
			continue
		}

		url := result.URL
		if result.FinalURL != "" {
			url = result.FinalURL
		}

		// Chain through all possible screenshot file field names
		screenshotFile := result.FileName
		if screenshotFile == "" {
			screenshotFile = result.Filename
		}
		if screenshotFile == "" {
			screenshotFile = result.Screenshot
		}
		if screenshotFile == "" {
			continue
		}

		filePath := screenshotFile
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(screenshotDir, filePath)
		}

		var subdomainID *string
		for _, sub := range input.Subdomains {
			if strings.Contains(url, sub.Hostname) {
				subdomainID = &sub.ID
				break
			}
		}

		// Prefer v3 response_code, fallback to status_code
		code := result.ResponseCode
		if code == 0 {
			code = result.StatusCode
		}
		var statusCode *int
		if code > 0 {
			statusCode = &code
		}
		var title *string
		if result.Title != "" {
			title = &result.Title
		}

		sink.AddScreenshot(ctx, &models.Screenshot{
			SubdomainID: subdomainID,
			URL:         url,
			FilePath:    filePath,
			StatusCode:  statusCode,
			Title:       title,
		})
		screenshotsParsed++
	}

	if linesRead > 0 && screenshotsParsed == 0 {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("WARNING: parsed %d JSONL lines but extracted 0 screenshots — possible format mismatch", linesRead))
	}

	return nil
}
