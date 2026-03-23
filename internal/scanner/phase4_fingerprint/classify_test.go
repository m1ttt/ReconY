package phase4

import (
	"net/http"
	"testing"

	"reconx/internal/models"
)

func TestClassifySiteType_PureReactSPA(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>App</title></head><body><div id="root"></div><script type="module" src="/static/js/main.abc123.js"></script></body></html>`
	siteType, evidence := classifySiteType(html, http.Header{})

	if siteType != models.SiteTypeSPA {
		t.Errorf("expected SPA, got %s (evidence: %v)", siteType, evidence)
	}
}

func TestClassifySiteType_NextJSHybrid(t *testing.T) {
	// Next.js produces both SPA-like and SSR signals
	html := `<!DOCTYPE html><html><head></head><body>
		<div id="__next"><h1>Server rendered content here with lots of text to make this page large enough</h1>
		<p>More content to ensure the HTML is substantial and not just a shell.</p>
		<p>Even more paragraphs with real content rendered server-side by Next.js.</p>
		<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt.</p>
		</div>
		<script src="/_next/static/chunks/main-abc123.js"></script>
		<script src="/_next/static/abc/pages/_app.js"></script>
		<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{}}}</script>
		<script>window.__NEXT_DATA__={"buildId":"test"}</script>
	</body></html>`
	// Pad HTML to > 5000 chars to trigger SSR bonus
	for len(html) < 6000 {
		html += "<!-- padding content to simulate real server-rendered page -->\n"
	}

	siteType, evidence := classifySiteType(html, http.Header{})

	if siteType != models.SiteTypeHybrid {
		t.Errorf("expected hybrid, got %s (evidence: %v)", siteType, evidence)
	}
}

func TestClassifySiteType_PureSSR(t *testing.T) {
	// SSR signals but weak SPA signals
	html := `<!DOCTYPE html><html><head></head><body>
		<div data-server-rendered="true">
		<h1>Server-rendered Vue page</h1>
		<p>Lots of server content here.</p>
		</div>
		<script>window.__NUXT__={config:{}}</script>
	</body></html>`
	for len(html) < 6000 {
		html += "<!-- padding -->\n"
	}

	siteType, _ := classifySiteType(html, http.Header{})

	if siteType != models.SiteTypeSSR {
		t.Errorf("expected SSR, got %s", siteType)
	}
}

func TestClassifySiteType_Classic(t *testing.T) {
	html := `<!DOCTYPE html><html><head></head><body>
		<h1>Welcome</h1>
		<form action="/login" method="post"><input name="user"/></form>
		<table><tr><td>Data</td></tr></table>
		<a href="/page.php?id=1">Link</a>
		<div class="wp-content"><img src="/wp-content/uploads/logo.png"/></div>
		<script src="/wp-includes/js/jquery.js"></script>
	</body></html>`

	siteType, _ := classifySiteType(html, http.Header{})

	if siteType != models.SiteTypeClassic {
		t.Errorf("expected classic, got %s", siteType)
	}
}

func TestClassifySiteType_API(t *testing.T) {
	html := `{"swagger":"2.0","info":{"title":"API"},"paths":{"/users":{"get":{}}}}`
	headers := http.Header{}
	headers.Set("Content-Type", "application/json; charset=utf-8")

	siteType, _ := classifySiteType(html, headers)

	if siteType != models.SiteTypeAPI {
		t.Errorf("expected api, got %s", siteType)
	}
}

func TestClassifySiteType_MinimalHTMLWithScripts(t *testing.T) {
	// Short HTML with script tag → SPA heuristic
	html := `<html><body><div id="root"></div><script src="/app.js"></script></body></html>`

	siteType, _ := classifySiteType(html, http.Header{})

	if siteType != models.SiteTypeSPA {
		t.Errorf("expected SPA for minimal HTML with scripts, got %s", siteType)
	}
}

func TestClassifyInfra_Vercel(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Vercel-Id", "iad1::abc123")

	infra := classifyInfra(headers, "example.com", nil)

	if infra != models.InfraTypeServerless {
		t.Errorf("expected serverless for Vercel, got %s", infra)
	}
}

func TestClassifyInfra_Fly(t *testing.T) {
	headers := http.Header{}
	headers.Set("Fly-Request-Id", "abc123")

	infra := classifyInfra(headers, "example.com", nil)

	if infra != models.InfraTypeContainer {
		t.Errorf("expected container for Fly.io, got %s", infra)
	}
}

func TestClassifyInfra_ManyPorts(t *testing.T) {
	headers := http.Header{}
	ports := []models.Port{
		{State: "open"}, {State: "open"}, {State: "open"},
		{State: "open"}, {State: "open"}, {State: "open"},
	}

	infra := classifyInfra(headers, "example.com", ports)

	if infra != models.InfraTypeBareMetal {
		t.Errorf("expected bare_metal for many open ports, got %s", infra)
	}
}

func TestClassifyInfra_FewPorts(t *testing.T) {
	headers := http.Header{}
	ports := []models.Port{
		{State: "open"},
	}

	infra := classifyInfra(headers, "example.com", ports)

	if infra != models.InfraTypeServerless {
		t.Errorf("expected serverless for few ports, got %s", infra)
	}
}
