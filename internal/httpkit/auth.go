package httpkit

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"reconx/internal/models"
)

// AuthSession manages an authenticated session for crawling.
type AuthSession struct {
	Credential *models.AuthCredential
	cookies    []*http.Cookie
	token      string
	headers    map[string]string
	jar        http.CookieJar
	mu         sync.RWMutex
}

// NewAuthSession creates a new auth session from a credential.
func NewAuthSession(cred *models.AuthCredential) *AuthSession {
	return &AuthSession{
		Credential: cred,
		headers:    make(map[string]string),
	}
}

// Login performs the authentication flow based on auth type.
func (a *AuthSession) Login(ctx context.Context, client *Client) error {
	switch a.Credential.AuthType {
	case models.AuthTypeBasic:
		user := ""
		pass := ""
		if a.Credential.Username != nil {
			user = *a.Credential.Username
		}
		if a.Credential.Password != nil {
			pass = *a.Credential.Password
		}
		a.token = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
		return nil

	case models.AuthTypeBearer:
		if a.Credential.Token != nil {
			a.token = "Bearer " + *a.Credential.Token
		}
		return nil

	case models.AuthTypeCookie:
		if a.Credential.Token != nil {
			header := http.Header{}
			header.Add("Cookie", *a.Credential.Token)
			req := &http.Request{Header: header}
			a.cookies = req.Cookies()
		}
		return nil

	case models.AuthTypeHeader:
		if a.Credential.HeaderName != nil && a.Credential.HeaderValue != nil {
			a.headers[*a.Credential.HeaderName] = *a.Credential.HeaderValue
		}
		return nil

	case models.AuthTypeForm:
		return a.formLogin(ctx, client)

	default:
		return nil
	}
}

func (a *AuthSession) formLogin(ctx context.Context, client *Client) error {
	if a.Credential.LoginURL == nil {
		return fmt.Errorf("login_url required for form auth")
	}

	jar, _ := cookiejar.New(nil)
	a.jar = jar

	// Step 1: GET the login page — extract CSRF, cookies, and HTML for form parsing
	page, err := a.fetchLoginPage(ctx, client, *a.Credential.LoginURL)
	if err != nil {
		return fmt.Errorf("fetching login page: %w", err)
	}

	// Step 2: Build the login body
	var body string
	var postURL string

	if a.Credential.LoginBody != nil {
		// User provided explicit body template — use it
		body = *a.Credential.LoginBody
		if a.Credential.Username != nil {
			body = strings.ReplaceAll(body, "{{username}}", *a.Credential.Username)
		}
		if a.Credential.Password != nil {
			body = strings.ReplaceAll(body, "{{password}}", *a.Credential.Password)
		}
		if page.csrf != "" {
			body = strings.ReplaceAll(body, "{{csrf}}", page.csrf)
			// Auto-inject CSRF if user didn't include {{csrf}} placeholder
			if !strings.Contains(body, page.csrf) {
				if strings.HasPrefix(strings.TrimSpace(body), "{") {
					body = strings.TrimSuffix(strings.TrimSpace(body), "}")
					body += fmt.Sprintf(",\"%s\":\"%s\"}", page.csrfKey, page.csrf)
				} else {
					if body != "" {
						body += "&"
					}
					body += url.QueryEscape(page.csrfKey) + "=" + url.QueryEscape(page.csrf)
				}
			}
		}
		postURL = *a.Credential.LoginURL
	} else {
		// No body template — auto-detect the login form from HTML
		form := parseLoginForm(page.html, *a.Credential.LoginURL)
		if form == nil {
			return fmt.Errorf("no login form found at %s (provide login_body to override)", *a.Credential.LoginURL)
		}

		postURL = form.action

		// Build form-encoded body from detected fields
		vals := url.Values{}
		for _, f := range form.fields {
			switch f.role {
			case fieldRoleUsername:
				if a.Credential.Username != nil {
					vals.Set(f.name, *a.Credential.Username)
				}
			case fieldRolePassword:
				if a.Credential.Password != nil {
					vals.Set(f.name, *a.Credential.Password)
				}
			case fieldRoleCSRF:
				if page.csrf != "" {
					vals.Set(f.name, page.csrf)
				} else if f.value != "" {
					vals.Set(f.name, f.value)
				}
			case fieldRoleHidden:
				// Preserve hidden fields (session tokens, honeypots, etc.)
				vals.Set(f.name, f.value)
			}
		}

		// Inject CSRF if form didn't have a dedicated field
		if page.csrf != "" && vals.Get(page.csrfKey) == "" {
			vals.Set(page.csrfKey, page.csrf)
		}

		body = vals.Encode()
	}

	// Step 3: POST the login request
	req, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating login request: %w", err)
	}

	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// Forward pre-auth cookies
	for _, c := range page.cookies {
		req.AddCookie(c)
	}

	// Inject CSRF as header too (some frameworks expect it)
	if page.csrf != "" {
		req.Header.Set("X-CSRF-Token", page.csrf)
		req.Header.Set("X-XSRF-Token", page.csrf)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()

	// Merge pre-auth cookies with post-login cookies
	a.mu.Lock()
	postCookies := resp.Cookies()
	merged := mergeCookies(page.cookies, postCookies)
	a.cookies = merged
	a.mu.Unlock()

	if len(a.cookies) == 0 {
		return fmt.Errorf("login HTTP %d but no cookies received", resp.StatusCode)
	}

	return nil
}

// CSRF token extraction patterns
var csrfPatterns = []struct {
	re    *regexp.Regexp
	field string // default field name if not extractable from HTML
}{
	// <input type="hidden" name="csrf_token" value="...">
	{regexp.MustCompile(`<input[^>]+name=["']([^"']*(?:csrf|_token|xsrf|authenticity)[^"']*)["'][^>]+value=["']([^"']+)["']`), ""},
	// Same but value before name
	{regexp.MustCompile(`<input[^>]+value=["']([^"']+)["'][^>]+name=["']([^"']*(?:csrf|_token|xsrf|authenticity)[^"']*)["']`), ""},
	// <meta name="csrf-token" content="...">
	{regexp.MustCompile(`<meta[^>]+name=["']csrf-token["'][^>]+content=["']([^"']+)["']`), "csrf_token"},
	// <meta content="..." name="csrf-token">
	{regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+name=["']csrf-token["']`), "csrf_token"},
	// window.csrfToken = "..."
	{regexp.MustCompile(`(?:csrfToken|csrf_token|_csrf|XSRF-TOKEN)\s*[:=]\s*["']([^"']+)["']`), "csrf_token"},
}

// loginPageResult holds everything extracted from GETting the login page.
type loginPageResult struct {
	html    string
	cookies []*http.Cookie
	csrf    string // CSRF token value
	csrfKey string // CSRF field name
}

// fetchLoginPage GETs the login page and extracts CSRF tokens and cookies.
func (a *AuthSession) fetchLoginPage(ctx context.Context, client *Client, loginURL string) (*loginPageResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", loginURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login page fetch failed: %w", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	resp.Body.Close()

	result := &loginPageResult{
		html:    string(body),
		cookies: resp.Cookies(),
	}

	// Check for CSRF cookie (e.g., XSRF-TOKEN, _csrf)
	for _, c := range result.cookies {
		lower := strings.ToLower(c.Name)
		if strings.Contains(lower, "csrf") || strings.Contains(lower, "xsrf") {
			result.csrf = c.Value
			result.csrfKey = c.Name
			return result, nil
		}
	}

	// Try HTML patterns
	for i, p := range csrfPatterns {
		m := p.re.FindStringSubmatch(result.html)
		if m == nil {
			continue
		}
		if p.field != "" {
			result.csrf = m[1]
			result.csrfKey = p.field
			return result, nil
		}
		if len(m) >= 3 {
			if i == 0 {
				result.csrf = m[2]
				result.csrfKey = m[1]
			} else {
				result.csrf = m[1]
				result.csrfKey = m[2]
			}
			return result, nil
		}
	}

	return result, nil // no CSRF found is not an error
}

// fetchCSRFToken is a backward-compatible wrapper around fetchLoginPage.
func (a *AuthSession) fetchCSRFToken(ctx context.Context, client *Client, loginURL string) (token, fieldName string, cookies []*http.Cookie, err error) {
	page, err := a.fetchLoginPage(ctx, client, loginURL)
	if err != nil {
		return "", "", nil, err
	}
	if page.csrf == "" {
		return "", "", page.cookies, fmt.Errorf("no CSRF token found")
	}
	return page.csrf, page.csrfKey, page.cookies, nil
}

// mergeCookies combines pre-auth and post-login cookies, with post-login taking precedence.
func mergeCookies(pre, post []*http.Cookie) []*http.Cookie {
	seen := make(map[string]*http.Cookie)
	for _, c := range pre {
		seen[c.Name] = c
	}
	for _, c := range post {
		seen[c.Name] = c // post-login overrides
	}
	result := make([]*http.Cookie, 0, len(seen))
	for _, c := range seen {
		result = append(result, c)
	}
	return result
}

// ApplyToRequest adds auth headers/cookies to an HTTP request.
func (a *AuthSession) ApplyToRequest(req *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.token != "" {
		req.Header.Set("Authorization", a.token)
	}
	for _, c := range a.cookies {
		req.AddCookie(c)
	}
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
}

// cookieHeader returns cookies as a header string (internal, no locking).
func (a *AuthSession) cookieHeader() string {
	var parts []string
	for _, c := range a.cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// CookieHeader returns cookies as a single Cookie header string.
func (a *AuthSession) CookieHeader() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cookieHeader()
}

// CLIHeaders returns auth as -H "Key: Value" CLI args for tools like katana/ffuf.
func (a *AuthSession) CLIHeaders() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var args []string
	if a.token != "" {
		args = append(args, "-H", "Authorization: "+a.token)
	}
	if len(a.cookies) > 0 {
		args = append(args, "-H", "Cookie: "+a.cookieHeader())
	}
	for k, v := range a.headers {
		args = append(args, "-H", k+": "+v)
	}
	return args
}
