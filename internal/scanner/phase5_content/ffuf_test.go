package phase5

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"reconx/internal/config"
	"reconx/internal/models"
)

// mockSink collects results for assertions.
type ffufMockSink struct {
	urls []models.DiscoveredURL
}

func (s *ffufMockSink) AddSubdomain(context.Context, *models.Subdomain) error           { return nil }
func (s *ffufMockSink) AddPort(context.Context, *models.Port) error                     { return nil }
func (s *ffufMockSink) AddTechnology(context.Context, *models.Technology) error          { return nil }
func (s *ffufMockSink) AddVulnerability(context.Context, *models.Vulnerability) error    { return nil }
func (s *ffufMockSink) AddDNSRecord(context.Context, *models.DNSRecord) error            { return nil }
func (s *ffufMockSink) AddWhoisRecord(context.Context, *models.WhoisRecord) error        { return nil }
func (s *ffufMockSink) AddHistoricalURL(context.Context, *models.HistoricalURL) error    { return nil }
func (s *ffufMockSink) AddParameter(context.Context, *models.Parameter) error            { return nil }
func (s *ffufMockSink) AddScreenshot(context.Context, *models.Screenshot) error          { return nil }
func (s *ffufMockSink) AddCloudAsset(context.Context, *models.CloudAsset) error          { return nil }
func (s *ffufMockSink) AddSecret(context.Context, *models.Secret) error                  { return nil }
func (s *ffufMockSink) AddSiteClassification(context.Context, *models.SiteClassification) error {
	return nil
}
func (s *ffufMockSink) LogLine(context.Context, string, string) {}
func (s *ffufMockSink) AddDiscoveredURL(_ context.Context, u *models.DiscoveredURL) error {
	s.urls = append(s.urls, *u)
	return nil
}

func writeFFUFJSON(t *testing.T, dir string, results []ffufResult) string {
	t.Helper()
	out := struct {
		Results []ffufResult `json:"results"`
	}{Results: results}
	data, _ := json.Marshal(out)
	path := filepath.Join(dir, "ffuf-out.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFFUFOutput_FiltersGarbageRedirects(t *testing.T) {
	dir := t.TempDir()
	base := "https://example.com"

	results := []ffufResult{
		// Good: 200 OK
		{Status: 200, Length: 1234, URL: base + "/admin", ContentType: "text/html"},
		// Garbage: 302 to /
		{Status: 302, Length: 0, URL: base + "/login", RedirectLocation: "/"},
		// Garbage: 301 to base URL
		{Status: 301, Length: 0, URL: base + "/old", RedirectLocation: "https://example.com/"},
		// Garbage: 302 empty location + 0 body
		{Status: 302, Length: 0, URL: base + "/foo"},
		// Good: 302 to specific path (not root)
		{Status: 302, Length: 50, URL: base + "/dashboard", RedirectLocation: "/app/dashboard"},
		// Good: 403 forbidden
		{Status: 403, Length: 300, URL: base + "/secret", ContentType: "text/html"},
		// Garbage: 307 to domain root
		{Status: 307, Length: 0, URL: base + "/bar", RedirectLocation: "https://example.com"},
	}

	path := writeFFUFJSON(t, dir, results)
	sink := &ffufMockSink{}
	parseFFUFOutput(context.Background(), path, base, sink)

	if len(sink.urls) != 3 {
		t.Fatalf("expected 3 results, got %d", len(sink.urls))
	}

	// /admin (200)
	if sink.urls[0].URL != base+"/admin" {
		t.Errorf("expected /admin, got %s", sink.urls[0].URL)
	}
	// /dashboard (302 to specific path)
	if sink.urls[1].URL != base+"/dashboard" {
		t.Errorf("expected /dashboard, got %s", sink.urls[1].URL)
	}
	if sink.urls[1].RedirectLocation == nil || *sink.urls[1].RedirectLocation != "/app/dashboard" {
		t.Error("expected redirect_location /app/dashboard")
	}
	// /secret (403)
	if sink.urls[2].URL != base+"/secret" {
		t.Errorf("expected /secret, got %s", sink.urls[2].URL)
	}
}

func TestIsGarbageRedirect_NonRedirectStatus(t *testing.T) {
	r := ffufResult{Status: 200, Length: 0, RedirectLocation: "/"}
	if isGarbageRedirect(r, "https://example.com") {
		t.Error("200 should never be garbage redirect")
	}
}

func TestIsGarbageRedirect_RedirectToSpecificPath(t *testing.T) {
	r := ffufResult{Status: 302, Length: 100, RedirectLocation: "/login?next=/admin"}
	if isGarbageRedirect(r, "https://example.com") {
		t.Error("redirect to specific path should not be garbage")
	}
}

func TestRemoveRedirectCodes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"200,201,301,302,403", "200,201,403"},
		{"200", "200"},
		{"301,302", "200"}, // all removed → fallback to 200
		{"200,201,301,302,303,307,308,403,500", "200,201,403,500"},
	}
	for _, tt := range tests {
		got := removeRedirectCodes(tt.input)
		if got != tt.expected {
			t.Errorf("removeRedirectCodes(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetFFUFStringOption(t *testing.T) {
	tc := config.ToolConfig{
		Options: map[string]any{
			"match_codes":  []any{200, 201, 403},
			"filter_codes": "404,500",
			"custom":       42,
		},
	}

	// []any → comma-separated string
	if got := getFFUFStringOption(tc, "match_codes", ""); got != "200,201,403" {
		t.Errorf("match_codes: got %q", got)
	}
	// string passthrough
	if got := getFFUFStringOption(tc, "filter_codes", ""); got != "404,500" {
		t.Errorf("filter_codes: got %q", got)
	}
	// missing key → fallback
	if got := getFFUFStringOption(tc, "missing", "default"); got != "default" {
		t.Errorf("missing: got %q", got)
	}
	// nil options → fallback
	tc2 := config.ToolConfig{}
	if got := getFFUFStringOption(tc2, "anything", "fb"); got != "fb" {
		t.Errorf("nil options: got %q", got)
	}
}

func TestGetFFUFBoolOption(t *testing.T) {
	tc := config.ToolConfig{
		Options: map[string]any{
			"auto_calibrate":   true,
			"follow_redirects": false,
			"str_true":         "true",
			"not_bool":         42,
		},
	}
	if !getFFUFBoolOption(tc, "auto_calibrate", false) {
		t.Error("expected true")
	}
	if getFFUFBoolOption(tc, "follow_redirects", true) {
		t.Error("expected false")
	}
	if !getFFUFBoolOption(tc, "str_true", false) {
		t.Error("expected string 'true' → true")
	}
	if getFFUFBoolOption(tc, "not_bool", false) {
		t.Error("expected non-bool → fallback false")
	}
	if !getFFUFBoolOption(tc, "missing", true) {
		t.Error("expected missing → fallback true")
	}
}
