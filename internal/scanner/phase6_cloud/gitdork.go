package phase6

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// GitDorkRunner searches GitHub for leaked secrets related to the target.
type GitDorkRunner struct{}

func (g *GitDorkRunner) Name() string         { return "gitdork" }
func (g *GitDorkRunner) Phase() engine.PhaseID { return engine.PhaseCloud }
func (g *GitDorkRunner) Check() error          { return nil }

func (g *GitDorkRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	token := input.Config.APIKeys.GithubToken
	if token == "" {
		sink.LogLine(ctx, "stderr", "GitHub token not configured, skipping gitdork")
		return nil
	}

	domain := input.Workspace.Domain

	// GitHub search dorks
	dorks := []struct {
		query    string
		category string
	}{
		{fmt.Sprintf("%s password", domain), "password"},
		{fmt.Sprintf("%s secret", domain), "secret"},
		{fmt.Sprintf("%s api_key", domain), "api_key"},
		{fmt.Sprintf("%s apikey", domain), "api_key"},
		{fmt.Sprintf("%s token", domain), "token"},
		{fmt.Sprintf("%s AWS_SECRET", domain), "aws_key"},
		{fmt.Sprintf("%s PRIVATE KEY", domain), "private_key"},
		{fmt.Sprintf("%s jdbc:", domain), "database"},
		{fmt.Sprintf("%s mongodb+srv", domain), "database"},
		{fmt.Sprintf("%s smtp", domain), "smtp"},
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Running %d GitHub dorks for %s", len(dorks), domain))
	client := httpkit.NewClient(input.Config)

	for _, dork := range dorks {
		if err := ctx.Err(); err != nil {
			return err
		}

		results, err := g.searchGitHub(ctx, client, token, dork.query)
		if err != nil {
			sink.LogLine(ctx, "stderr", fmt.Sprintf("GitHub search failed: %v", err))
			time.Sleep(5 * time.Second)
			continue
		}

		for _, item := range results {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("[%s] %s: %s", dork.category, item.Repository, item.Path))
			sink.AddSecret(ctx, &models.Secret{
				SourceURL:  item.HTMLURL,
				SecretType: dork.category,
				Value:      fmt.Sprintf("%s/%s", item.Repository, item.Path),
				Source:     "gitdork",
				Severity:   models.SeverityHigh,
			})
		}

		// GitHub rate limit: 30 requests/min for search
		time.Sleep(2 * time.Second)
	}

	return nil
}

type ghSearchItem struct {
	Repository string
	Path       string
	HTMLURL    string
}

func (g *GitDorkRunner) searchGitHub(ctx context.Context, client *httpkit.Client, token, query string) ([]ghSearchItem, error) {
	u := fmt.Sprintf("https://api.github.com/search/code?q=%s&per_page=10", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			HTMLURL    string `json:"html_url"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var items []ghSearchItem
	for _, item := range result.Items {
		items = append(items, ghSearchItem{
			Repository: item.Repository.FullName,
			Path:       item.Path,
			HTMLURL:    item.HTMLURL,
		})
	}

	return items, nil
}
