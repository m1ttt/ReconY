package phase6

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
	"reconx/internal/scanner"
)

// JSSecretsRunner scans JavaScript files for hardcoded secrets using regex patterns.
type JSSecretsRunner struct {
	authSessions []*httpkit.AuthSession
}

func (j *JSSecretsRunner) Name() string         { return "js_secrets" }
func (j *JSSecretsRunner) Phase() engine.PhaseID { return engine.PhaseCloud }
func (j *JSSecretsRunner) Check() error          { return nil }

func (j *JSSecretsRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	targets := scanner.BuildHTTPTargets(input.Subdomains, input.Classifications, input.Workspace.Domain)

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Scanning JS for secrets on %d targets", len(targets)))
	client := httpkit.NewClient(input.Config)
	j.authSessions = input.AuthSessions

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		j.scanTarget(ctx, client, target, sink)
	}

	return nil
}

func (j *JSSecretsRunner) scanTarget(ctx context.Context, client *httpkit.Client, target string, sink engine.ResultSink) {
	// Fetch main page to find JS files
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	for _, sess := range j.authSessions {
		sess.ApplyToRequest(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	resp.Body.Close()

	html := string(body)

	// Extract JS URLs from HTML
	jsRe := regexp.MustCompile(`(?:src|href)=["']([^"']*\.js[^"']*)["']`)
	matches := jsRe.FindAllStringSubmatch(html, -1)

	var jsURLs []string
	for _, match := range matches {
		jsURL := match[1]
		if strings.HasPrefix(jsURL, "//") {
			jsURL = "https:" + jsURL
		} else if strings.HasPrefix(jsURL, "/") {
			jsURL = target + jsURL
		} else if !strings.HasPrefix(jsURL, "http") {
			jsURL = target + "/" + jsURL
		}
		jsURLs = append(jsURLs, jsURL)
	}

	// Also scan the HTML itself
	j.scanContent(ctx, target, html, sink)

	// Scan each JS file
	for _, jsURL := range jsURLs {
		if err := ctx.Err(); err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, "GET", jsURL, nil)
		if err != nil {
			continue
		}
		for _, sess := range j.authSessions {
			sess.ApplyToRequest(req)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		jsBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		resp.Body.Close()

		j.scanContent(ctx, jsURL, string(jsBody), sink)
	}
}

var secretPatterns = []struct {
	name     string
	pattern  *regexp.Regexp
	severity models.Severity
}{
	{"aws_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), models.SeverityCritical},
	{"aws_secret", regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key[\s=:"']+([A-Za-z0-9/+=]{40})`), models.SeverityCritical},
	{"google_api", regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), models.SeverityHigh},
	{"github_token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`), models.SeverityCritical},
	{"slack_token", regexp.MustCompile(`xox[baprs]-[0-9A-Za-z\-]{10,}`), models.SeverityHigh},
	{"stripe_key", regexp.MustCompile(`(?:sk|pk)_(?:test|live)_[0-9a-zA-Z]{24,}`), models.SeverityCritical},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_.+/=]*`), models.SeverityMedium},
	{"private_key", regexp.MustCompile(`-----BEGIN (?:RSA |EC )?PRIVATE KEY-----`), models.SeverityCritical},
	{"bearer_token", regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`), models.SeverityMedium},
	{"basic_auth", regexp.MustCompile(`(?i)basic\s+[A-Za-z0-9+/]{20,}={0,2}`), models.SeverityHigh},
	{"firebase", regexp.MustCompile(`(?i)firebase[a-z.]*\.com`), models.SeverityLow},
	{"mailgun", regexp.MustCompile(`key-[0-9a-zA-Z]{32}`), models.SeverityHigh},
	{"twilio", regexp.MustCompile(`SK[0-9a-fA-F]{32}`), models.SeverityHigh},
	{"sendgrid", regexp.MustCompile(`SG\.[a-zA-Z0-9_-]{22}\.[a-zA-Z0-9_-]{43}`), models.SeverityHigh},
}

func (j *JSSecretsRunner) scanContent(ctx context.Context, sourceURL, content string, sink engine.ResultSink) {
	for _, sp := range secretPatterns {
		matches := sp.pattern.FindAllString(content, 5) // Limit matches per pattern
		for _, match := range matches {
			// Truncate long matches
			value := match
			if len(value) > 100 {
				value = value[:100] + "..."
			}

			// Get context (surrounding text)
			idx := strings.Index(content, match)
			ctxStart := max(0, idx-50)
			ctxEnd := min(len(content), idx+len(match)+50)
			contextStr := content[ctxStart:ctxEnd]
			contextStr = strings.ReplaceAll(contextStr, "\n", " ")

			sink.AddSecret(ctx, &models.Secret{
				SourceURL:  sourceURL,
				SecretType: sp.name,
				Value:      value,
				Context:    &contextStr,
				Source:     "js_secrets",
				Severity:   sp.severity,
			})

			sink.LogLine(ctx, "stdout", fmt.Sprintf("[!] %s found in %s: %s", sp.name, sourceURL, value[:min(len(value), 30)]))
		}
	}
}
