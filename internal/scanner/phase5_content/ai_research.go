package phase5

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"reconx/internal/engine"
	"reconx/internal/models"
	"reconx/internal/scanner"
)

// AIResearchRunner uses an external LangGraph+OpenAI microservice
// to perform deep research on subdomains or domains found in previous phases.
type AIResearchRunner struct{}

func (r *AIResearchRunner) Name() string { return "ai_research" }

func (r *AIResearchRunner) Phase() engine.PhaseID { return engine.PhaseContent }

func (r *AIResearchRunner) Check() error {
	// Optional: we could do a quick health check to the python microservice here.
	// We'll just assume it's available or will fail gracefully.
	return nil
}

// Request payload for the python ai-service
type aiRequest struct {
	Query string `json:"query"`
}

// Response payload from the python ai-service
type aiResponse struct {
	Result string `json:"result"`
}

func (r *AIResearchRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)
	if len(targets) == 0 {
		return fmt.Errorf("no targets available for AI research")
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Starting AI research on %d targets via local microservice...", len(targets)))

	client := &http.Client{
		Timeout: 5 * time.Minute, // AI requests can take a while
	}

	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		query := fmt.Sprintf("Find the latest security vulnerabilities, exposed assets, and technology stack information for %s", t)

		reqBody, err := json.Marshal(aiRequest{Query: query})
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to marshal request for %s: %v", t, err))
			continue
		}

		// Assuming the python service is running on localhost:8000
		req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8000/research", bytes.NewBuffer(reqBody))
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to create request for %s: %v", t, err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to reach ai-service for %s. Is it running? Error: %v", t, err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			sink.LogLine(ctx, "stderr", fmt.Sprintf("ai-service returned status %d for %s: %s", resp.StatusCode, t, string(bodyBytes)))
			continue
		}

		var result aiResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to decode ai-service response for %s: %v", t, err))
			continue
		}

		// Store the result as evidence in site_classifications or a generic finding
		evidence := result.Result
		err = sink.AddSiteClassification(ctx, &models.SiteClassification{
			URL:      t,
			Evidence: &evidence,
			SiteType: models.SiteTypeUnknown,
		})
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("Failed to save classification for %s: %v", t, err))
		} else {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("Successfully generated AI research for %s", t))
		}
	}

	return nil
}
