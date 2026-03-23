package phase4

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reconx/internal/engine"
	"reconx/internal/httpkit"
	"reconx/internal/models"
)

// WAFDetectRunner detects WAF, CDN, and reverse proxy presence.
type WAFDetectRunner struct{}

func (w *WAFDetectRunner) Name() string         { return "waf_detect" }
func (w *WAFDetectRunner) Phase() engine.PhaseID { return engine.PhaseFingerprint }
func (w *WAFDetectRunner) Check() error          { return nil }

func (w *WAFDetectRunner) Run(ctx context.Context, input *engine.PhaseInput, sink engine.ResultSink) error {
	var targets []string
	// Prefer alive subdomains, fall back to all provided, then domain
	for _, sub := range input.Subdomains {
		if sub.IsAlive {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		for _, sub := range input.Subdomains {
			targets = append(targets, sub.Hostname)
		}
	}
	if len(targets) == 0 {
		targets = []string{input.Workspace.Domain}
	}

	sink.LogLine(ctx, "stdout", fmt.Sprintf("Detecting WAF/CDN on %d targets", len(targets)))
	client := httpkit.NewClientWithRedirects(input.Config, 3)

	for _, host := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.detectHost(ctx, client, host, input, input.AuthSessions, sink)
	}

	return nil
}

func (w *WAFDetectRunner) detectHost(ctx context.Context, client *httpkit.Client, host string, input *engine.PhaseInput, authSessions []*httpkit.AuthSession, sink engine.ResultSink) {
	var probeErrors []string

	for _, scheme := range []string{"https", "http"} {
		url := fmt.Sprintf("%s://%s", scheme, host)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: request build failed: %v", url, err))
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; reconx/1.0)")
		for _, sess := range authSessions {
			sess.ApplyToRequest(req)
		}

		resp, err := client.Do(req)
		if err != nil {
			probeErrors = append(probeErrors, fmt.Sprintf("%s: request failed: %v", url, err))
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()

		var subdomainID *string
		for _, sub := range input.Subdomains {
			if sub.Hostname == host {
				subdomainID = &sub.ID
				break
			}
		}

		waf := detectWAF(resp.Header, string(body))
		cdn := detectCDN(resp.Header)

		var wafPtr, cdnPtr *string
		if waf != "" {
			wafPtr = &waf
			cat := "waf"
			sink.AddTechnology(ctx, &models.Technology{
				SubdomainID: subdomainID,
				URL:         url,
				Name:        waf,
				Category:    &cat,
				Confidence:  85,
			})
			sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: WAF detected: %s", host, waf))
		}
		if cdn != "" {
			cdnPtr = &cdn
			cat := "cdn"
			sink.AddTechnology(ctx, &models.Technology{
				SubdomainID: subdomainID,
				URL:         url,
				Name:        cdn,
				Category:    &cat,
				Confidence:  90,
			})
			sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: CDN detected: %s", host, cdn))
		}

		// Always write classification (even when no WAF/CDN found)
		sink.AddSiteClassification(ctx, &models.SiteClassification{
			SubdomainID: subdomainID,
			URL:         url,
			SiteType:    models.SiteTypeUnknown,
			WAFDetected: wafPtr,
			CDNDetected: cdnPtr,
		})
		if waf == "" && cdn == "" {
			sink.LogLine(ctx, "stdout", fmt.Sprintf("%s: no WAF/CDN detected", host))
		}

		return // Use first successful scheme
	}

	if len(probeErrors) > 0 {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("%s: WAF/CDN probe failed (%s)", host, strings.Join(probeErrors, " | ")))
	} else {
		sink.LogLine(ctx, "stderr", fmt.Sprintf("%s: WAF/CDN probe failed with unknown error", host))
	}
}

var wafSignatures = map[string][]string{
	"Cloudflare":    {"cf-ray", "cf-cache-status", "__cfduid"},
	"AWS WAF":       {"x-amzn-requestid", "x-amz-cf-id"},
	"Akamai":        {"x-akamai-transformed", "akamai-origin-hop"},
	"Sucuri":        {"x-sucuri-id", "x-sucuri-cache"},
	"Imperva":       {"x-iinfo", "x-cdn"},
	"F5 BigIP":      {"x-wa-info", "x-cnection"},
	"ModSecurity":   {"mod_security"},
	"Barracuda":     {"barra_counter_session"},
	"Fortinet":      {"fortiwafsid"},
}

func detectWAF(headers http.Header, body string) string {
	headerStr := strings.ToLower(fmt.Sprintf("%v", headers))

	for waf, sigs := range wafSignatures {
		for _, sig := range sigs {
			if strings.Contains(headerStr, strings.ToLower(sig)) {
				return waf
			}
		}
	}

	// Body-based detection
	bodyLower := strings.ToLower(body)
	if strings.Contains(bodyLower, "attention required! | cloudflare") {
		return "Cloudflare"
	}
	if strings.Contains(bodyLower, "access denied") && strings.Contains(bodyLower, "incapsula") {
		return "Imperva Incapsula"
	}

	return ""
}

var cdnSignatures = map[string][]string{
	"Cloudflare":  {"cf-ray"},
	"CloudFront":  {"x-amz-cf-id", "x-amz-cf-pop"},
	"Fastly":      {"x-served-by", "x-fastly-request-id"},
	"Akamai":      {"x-akamai-transformed"},
	"StackPath":   {"x-sp-"},
	"KeyCDN":      {"x-edge-location"},
	"Vercel":      {"x-vercel-id"},
	"Netlify":     {"x-nf-request-id"},
}

func detectCDN(headers http.Header) string {
	headerStr := strings.ToLower(fmt.Sprintf("%v", headers))

	for cdn, sigs := range cdnSignatures {
		for _, sig := range sigs {
			if strings.Contains(headerStr, strings.ToLower(sig)) {
				return cdn
			}
		}
	}

	// Server header check
	server := strings.ToLower(headers.Get("Server"))
	switch {
	case strings.Contains(server, "cloudflare"):
		return "Cloudflare"
	case strings.Contains(server, "cloudfront"):
		return "CloudFront"
	case strings.Contains(server, "netlify"):
		return "Netlify"
	}

	return ""
}
