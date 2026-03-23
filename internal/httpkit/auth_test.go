package httpkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"reconx/internal/models"
)

func strPtr(s string) *string { return &s }

func TestAuthSession_BasicAuth(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeBasic,
		Username: strPtr("admin"),
		Password: strPtr("secret"),
	}

	sess := NewAuthSession(cred)
	if err := sess.Login(context.Background(), nil); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	if sess.token != expected {
		t.Errorf("token = %q, want %q", sess.token, expected)
	}

	// Verify ApplyToRequest
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	sess.ApplyToRequest(req)
	if got := req.Header.Get("Authorization"); got != expected {
		t.Errorf("Authorization header = %q, want %q", got, expected)
	}
}

func TestAuthSession_BearerToken(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeBearer,
		Token:    strPtr("eyJhbGciOiJIUzI1NiJ9.test"),
	}

	sess := NewAuthSession(cred)
	if err := sess.Login(context.Background(), nil); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	sess.ApplyToRequest(req)

	want := "Bearer eyJhbGciOiJIUzI1NiJ9.test"
	if got := req.Header.Get("Authorization"); got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestAuthSession_Cookie(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeCookie,
		Token:    strPtr("session=abc123; csrf=xyz789"),
	}

	sess := NewAuthSession(cred)
	if err := sess.Login(context.Background(), nil); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if len(sess.cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(sess.cookies))
	}

	// Verify CookieHeader
	header := sess.CookieHeader()
	if !strings.Contains(header, "session=abc123") || !strings.Contains(header, "csrf=xyz789") {
		t.Errorf("CookieHeader = %q, missing expected cookies", header)
	}

	// Verify ApplyToRequest adds cookies
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	sess.ApplyToRequest(req)
	cookies := req.Cookies()
	if len(cookies) != 2 {
		t.Errorf("expected 2 cookies on request, got %d", len(cookies))
	}
}

func TestAuthSession_CustomHeader(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType:    models.AuthTypeHeader,
		HeaderName:  strPtr("X-API-Key"),
		HeaderValue: strPtr("my-secret-key-123"),
	}

	sess := NewAuthSession(cred)
	if err := sess.Login(context.Background(), nil); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	sess.ApplyToRequest(req)

	if got := req.Header.Get("X-API-Key"); got != "my-secret-key-123" {
		t.Errorf("X-API-Key = %q, want my-secret-key-123", got)
	}
}

func TestAuthSession_FormLogin(t *testing.T) {
	// Mock login server: GET returns no CSRF, POST validates credentials
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// CSRF prefetch — no token, just return empty page
			w.Write([]byte("<html>Login</html>"))
			return
		}

		// Verify body has substituted values
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["email"] != "admin@test.com" {
			t.Errorf("email = %q, want admin@test.com", body["email"])
		}
		if body["password"] != "secret123" {
			t.Errorf("password = %q, want secret123", body["password"])
		}

		// Return cookies
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "tok_abc"})
		http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "csrf_xyz"})
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	loginBody := `{"email":"{{username}}","password":"{{password}}"}`
	cred := &models.AuthCredential{
		AuthType:  models.AuthTypeForm,
		Username:  strPtr("admin@test.com"),
		Password:  strPtr("secret123"),
		LoginURL:  strPtr(server.URL + "/api/login"),
		LoginBody: strPtr(loginBody),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	if err := sess.Login(context.Background(), client); err != nil {
		t.Fatalf("Form login failed: %v", err)
	}

	if len(sess.cookies) != 2 {
		t.Fatalf("expected 2 cookies after login, got %d", len(sess.cookies))
	}

	header := sess.CookieHeader()
	if !strings.Contains(header, "session=tok_abc") {
		t.Errorf("missing session cookie in %q", header)
	}
}

func TestAuthSession_FormLogin_NoCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer server.Close()

	cred := &models.AuthCredential{
		AuthType: models.AuthTypeForm,
		Username: strPtr("user"),
		Password: strPtr("wrong"),
		LoginURL: strPtr(server.URL + "/login"),
		LoginBody: strPtr(`user={{username}}&pass={{password}}`),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	err := sess.Login(context.Background(), client)

	if err == nil {
		t.Fatal("expected error for login with no cookies")
	}
	if !strings.Contains(err.Error(), "no cookies received") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthSession_FormLogin_MissingURL(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeForm,
	}

	sess := NewAuthSession(cred)
	err := sess.Login(context.Background(), nil)

	if err == nil {
		t.Fatal("expected error for form login without URL")
	}
	if !strings.Contains(err.Error(), "login_url required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuthSession_CLIHeaders_Basic(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeBasic,
		Username: strPtr("user"),
		Password: strPtr("pass"),
	}

	sess := NewAuthSession(cred)
	sess.Login(context.Background(), nil)

	args := sess.CLIHeaders()
	if len(args) != 2 {
		t.Fatalf("expected 2 args (-H and value), got %d: %v", len(args), args)
	}
	if args[0] != "-H" {
		t.Errorf("args[0] = %q, want -H", args[0])
	}
	if !strings.HasPrefix(args[1], "Authorization: Basic ") {
		t.Errorf("args[1] = %q, want Authorization: Basic ...", args[1])
	}
}

func TestAuthSession_CLIHeaders_CookiePlusBearer(t *testing.T) {
	// Manually set up a session with both token and cookies
	sess := &AuthSession{
		Credential: &models.AuthCredential{AuthType: models.AuthTypeBearer},
		headers:    make(map[string]string),
		token:      "Bearer abc",
		cookies: []*http.Cookie{
			{Name: "sid", Value: "123"},
		},
	}

	args := sess.CLIHeaders()
	// Should have: -H "Authorization: Bearer abc" -H "Cookie: sid=123"
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}

	found := map[string]bool{}
	for i := 0; i < len(args); i += 2 {
		found[args[i+1]] = true
	}
	if !found["Authorization: Bearer abc"] {
		t.Error("missing Authorization header in CLI args")
	}
	if !found["Cookie: sid=123"] {
		t.Error("missing Cookie header in CLI args")
	}
}

func TestAuthSession_FormLogin_WithCSRFInput(t *testing.T) {
	// Server that serves a login page with CSRF token on GET, validates it on POST
	csrfToken := "tok_csrf_abc123xyz"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			http.SetCookie(w, &http.Cookie{Name: "preflight", Value: "pre123"})
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><form action="/login" method="POST">
				<input type="hidden" name="csrf_token" value="%s">
				<input name="username"><input name="password">
			</form></html>`, csrfToken)
			return
		}
		// POST — verify CSRF token was included
		r.ParseForm()
		got := r.FormValue("csrf_token")
		if got != csrfToken {
			// Also check JSON body
			t.Errorf("csrf_token = %q, want %q", got, csrfToken)
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "authenticated_abc"})
		w.WriteHeader(200)
	}))
	defer server.Close()

	cred := &models.AuthCredential{
		AuthType:  models.AuthTypeForm,
		Username:  strPtr("admin"),
		Password:  strPtr("pass123"),
		LoginURL:  strPtr(server.URL + "/login"),
		LoginBody: strPtr("username={{username}}&password={{password}}"),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	if err := sess.Login(context.Background(), client); err != nil {
		t.Fatalf("CSRF form login failed: %v", err)
	}

	header := sess.CookieHeader()
	if !strings.Contains(header, "session=authenticated_abc") {
		t.Errorf("missing session cookie, got: %q", header)
	}
	// Pre-auth cookie should also be preserved
	if !strings.Contains(header, "preflight=pre123") {
		t.Errorf("missing pre-auth cookie, got: %q", header)
	}
}

func TestAuthSession_FormLogin_WithCSRFCookie(t *testing.T) {
	// Server returns CSRF as a cookie (like Angular/Laravel double-submit pattern)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			http.SetCookie(w, &http.Cookie{Name: "XSRF-TOKEN", Value: "xsrf_cookie_val"})
			w.Write([]byte("<html><body>Login</body></html>"))
			return
		}
		// POST — check X-XSRF-Token header
		if r.Header.Get("X-XSRF-Token") != "xsrf_cookie_val" {
			t.Errorf("X-XSRF-Token = %q, want xsrf_cookie_val", r.Header.Get("X-XSRF-Token"))
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "logged_in"})
		w.WriteHeader(200)
	}))
	defer server.Close()

	cred := &models.AuthCredential{
		AuthType:  models.AuthTypeForm,
		Username:  strPtr("user"),
		Password:  strPtr("pass"),
		LoginURL:  strPtr(server.URL + "/login"),
		LoginBody: strPtr(`{"user":"{{username}}","pass":"{{password}}"}`),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	if err := sess.Login(context.Background(), client); err != nil {
		t.Fatalf("CSRF cookie login failed: %v", err)
	}

	header := sess.CookieHeader()
	if !strings.Contains(header, "session=logged_in") {
		t.Errorf("missing session cookie, got: %q", header)
	}
}

func TestAuthSession_FormLogin_WithCSRFMeta(t *testing.T) {
	// Server returns CSRF in a <meta> tag (like Rails)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`<html><head><meta name="csrf-token" content="meta_csrf_123"></head><body></body></html>`))
			return
		}
		if r.Header.Get("X-CSRF-Token") != "meta_csrf_123" {
			t.Errorf("X-CSRF-Token = %q, want meta_csrf_123", r.Header.Get("X-CSRF-Token"))
		}
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "ok"})
		w.WriteHeader(200)
	}))
	defer server.Close()

	cred := &models.AuthCredential{
		AuthType:  models.AuthTypeForm,
		Username:  strPtr("u"),
		Password:  strPtr("p"),
		LoginURL:  strPtr(server.URL),
		LoginBody: strPtr("u={{username}}&p={{password}}"),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	if err := sess.Login(context.Background(), client); err != nil {
		t.Fatalf("CSRF meta login failed: %v", err)
	}

	header := sess.CookieHeader()
	if !strings.Contains(header, "sid=ok") {
		t.Errorf("missing session cookie, got: %q", header)
	}
}

func TestAuthSession_FormLogin_CSRFPlaceholder(t *testing.T) {
	// User explicitly uses {{csrf}} in body template
	csrfToken := "explicit_csrf_999"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprintf(w, `<input type="hidden" name="_token" value="%s">`, csrfToken)
			return
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["_csrf"] != csrfToken {
			t.Errorf("_csrf = %q, want %q", body["_csrf"], csrfToken)
		}
		http.SetCookie(w, &http.Cookie{Name: "s", Value: "ok"})
		w.WriteHeader(200)
	}))
	defer server.Close()

	cred := &models.AuthCredential{
		AuthType:  models.AuthTypeForm,
		Username:  strPtr("u"),
		Password:  strPtr("p"),
		LoginURL:  strPtr(server.URL),
		LoginBody: strPtr(`{"user":"{{username}}","pass":"{{password}}","_csrf":"{{csrf}}"}`),
	}

	sess := NewAuthSession(cred)
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	if err := sess.Login(context.Background(), client); err != nil {
		t.Fatalf("CSRF placeholder login failed: %v", err)
	}
}

func TestFetchCSRFToken_InputHidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<form><input type="hidden" name="csrf_token" value="abc123"></form>`))
	}))
	defer server.Close()

	sess := &AuthSession{Credential: &models.AuthCredential{}, headers: make(map[string]string)}
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	token, field, _, err := sess.fetchCSRFToken(context.Background(), client, server.URL)
	if err != nil {
		t.Fatalf("fetchCSRFToken failed: %v", err)
	}
	if token != "abc123" {
		t.Errorf("token = %q, want abc123", token)
	}
	if field != "csrf_token" {
		t.Errorf("field = %q, want csrf_token", field)
	}
}

func TestFetchCSRFToken_NoneFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>No CSRF here</body></html>`))
	}))
	defer server.Close()

	sess := &AuthSession{Credential: &models.AuthCredential{}, headers: make(map[string]string)}
	client := &Client{inner: server.Client(), rateLimiter: NewHostRateLimiter(100), maxRetries: 1}
	_, _, _, err := sess.fetchCSRFToken(context.Background(), client, server.URL)
	if err == nil {
		t.Error("expected error when no CSRF token found")
	}
}

func TestMergeCookies(t *testing.T) {
	pre := []*http.Cookie{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "old"},
	}
	post := []*http.Cookie{
		{Name: "b", Value: "new"},
		{Name: "c", Value: "3"},
	}
	merged := mergeCookies(pre, post)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged cookies, got %d", len(merged))
	}
	m := map[string]string{}
	for _, c := range merged {
		m[c.Name] = c.Value
	}
	if m["a"] != "1" {
		t.Errorf("cookie a = %q, want 1", m["a"])
	}
	if m["b"] != "new" {
		t.Errorf("cookie b = %q, want new (post should override)", m["b"])
	}
	if m["c"] != "3" {
		t.Errorf("cookie c = %q, want 3", m["c"])
	}
}

func TestAuthSession_NoneType(t *testing.T) {
	cred := &models.AuthCredential{
		AuthType: models.AuthTypeNone,
	}

	sess := NewAuthSession(cred)
	if err := sess.Login(context.Background(), nil); err != nil {
		t.Fatalf("Login for none type should not fail: %v", err)
	}

	args := sess.CLIHeaders()
	if len(args) != 0 {
		t.Errorf("expected 0 CLI args for none auth, got %d", len(args))
	}
}
