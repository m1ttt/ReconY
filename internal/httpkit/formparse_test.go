package httpkit

import (
	"testing"
)

func TestParseLoginForm_BasicHTMLForm(t *testing.T) {
	html := `
	<html><body>
	<form action="/login" method="POST">
		<input type="text" name="username" />
		<input type="password" name="password" />
		<input type="submit" value="Login" />
	</form>
	</body></html>`

	form := parseLoginForm(html, "https://example.com/login")
	if form == nil {
		t.Fatal("expected form, got nil")
	}
	if form.action != "https://example.com/login" {
		t.Errorf("action = %q", form.action)
	}
	if len(form.fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(form.fields))
	}
	if form.fields[0].role != fieldRoleUsername {
		t.Errorf("field[0] role = %d, want username", form.fields[0].role)
	}
	if form.fields[1].role != fieldRolePassword {
		t.Errorf("field[1] role = %d, want password", form.fields[1].role)
	}
}

func TestParseLoginForm_EmailField(t *testing.T) {
	html := `
	<form action="/auth" method="post">
		<input type="email" name="correo" />
		<input type="password" name="clave" />
	</form>`

	form := parseLoginForm(html, "https://example.com/auth")
	if form == nil {
		t.Fatal("expected form, got nil")
	}
	if form.fields[0].role != fieldRoleUsername {
		t.Errorf("email field should be username, got role %d", form.fields[0].role)
	}
}

func TestParseLoginForm_WithCSRFAndHiddenFields(t *testing.T) {
	html := `
	<form action="/login" method="POST">
		<input type="hidden" name="csrf_token" value="abc123" />
		<input type="hidden" name="redirect" value="/dashboard" />
		<input type="text" name="user" />
		<input type="password" name="pass" />
	</form>`

	form := parseLoginForm(html, "https://example.com/login")
	if form == nil {
		t.Fatal("expected form")
	}
	if len(form.fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(form.fields))
	}

	// csrf_token → fieldRoleCSRF
	if form.fields[0].role != fieldRoleCSRF {
		t.Errorf("csrf_token role = %d, want CSRF", form.fields[0].role)
	}
	if form.fields[0].value != "abc123" {
		t.Errorf("csrf value = %q", form.fields[0].value)
	}

	// redirect → fieldRoleHidden
	if form.fields[1].role != fieldRoleHidden {
		t.Errorf("redirect role = %d, want hidden", form.fields[1].role)
	}

	// user → username
	if form.fields[2].role != fieldRoleUsername {
		t.Errorf("user role = %d, want username", form.fields[2].role)
	}

	// pass → password
	if form.fields[3].role != fieldRolePassword {
		t.Errorf("pass role = %d, want password", form.fields[3].role)
	}
}

func TestParseLoginForm_RelativeAction(t *testing.T) {
	html := `<form action="/api/auth/login" method="POST">
		<input type="text" name="email" />
		<input type="password" name="password" />
	</form>`

	form := parseLoginForm(html, "https://app.example.com/login")
	if form == nil {
		t.Fatal("expected form")
	}
	if form.action != "https://app.example.com/api/auth/login" {
		t.Errorf("action = %q", form.action)
	}
}

func TestParseLoginForm_NoPasswordField(t *testing.T) {
	html := `<form action="/search" method="GET">
		<input type="text" name="q" />
		<input type="submit" value="Search" />
	</form>`

	form := parseLoginForm(html, "https://example.com/search")
	if form != nil {
		t.Error("search form should not be detected as login form")
	}
}

func TestParseLoginForm_MultipleForms(t *testing.T) {
	html := `
	<form action="/search" method="GET">
		<input type="text" name="q" />
	</form>
	<form action="/login" method="POST">
		<input type="email" name="email" />
		<input type="password" name="password" />
	</form>
	<form action="/subscribe" method="POST">
		<input type="email" name="newsletter_email" />
		<input type="submit" value="Subscribe" />
	</form>`

	form := parseLoginForm(html, "https://example.com/")
	if form == nil {
		t.Fatal("expected login form")
	}
	if form.action != "https://example.com/login" {
		t.Errorf("action = %q, want /login", form.action)
	}
}

func TestParseLoginForm_UnknownFieldBecomesUsername(t *testing.T) {
	// Field name "entrada" doesn't match any username pattern,
	// but it's the only text field before password → should become username
	html := `
	<form action="/login" method="POST">
		<input type="text" name="entrada" />
		<input type="password" name="clave" />
	</form>`

	form := parseLoginForm(html, "https://example.com/login")
	if form == nil {
		t.Fatal("expected form")
	}
	if form.fields[0].role != fieldRoleUsername {
		t.Errorf("text field before password should default to username, got role %d", form.fields[0].role)
	}
}

func TestParseLoginForm_DjangoStyleCSRF(t *testing.T) {
	html := `
	<form action="/accounts/login/" method="POST">
		<input type="hidden" name="csrfmiddlewaretoken" value="WKsXdHq3abc" />
		<input type="text" name="username" />
		<input type="password" name="password" />
	</form>`

	form := parseLoginForm(html, "https://example.com/accounts/login/")
	if form == nil {
		t.Fatal("expected form")
	}
	if form.fields[0].role != fieldRoleCSRF {
		t.Errorf("csrfmiddlewaretoken should be CSRF, got %d", form.fields[0].role)
	}
}

func TestParseLoginForm_RailsStyleCSRF(t *testing.T) {
	html := `
	<form action="/session" method="POST">
		<input type="hidden" name="authenticity_token" value="xyz789==" />
		<input type="text" name="login" />
		<input type="password" name="password" />
	</form>`

	form := parseLoginForm(html, "https://example.com/session")
	if form == nil {
		t.Fatal("expected form")
	}
	if form.fields[0].role != fieldRoleCSRF {
		t.Errorf("authenticity_token should be CSRF, got %d", form.fields[0].role)
	}
}

func TestParseLoginForm_EmptyAction(t *testing.T) {
	html := `
	<form method="POST">
		<input type="text" name="user" />
		<input type="password" name="pass" />
	</form>`

	form := parseLoginForm(html, "https://example.com/login")
	if form == nil {
		t.Fatal("expected form")
	}
	// No action → should use the page URL
	if form.action != "https://example.com/login" {
		t.Errorf("action = %q, want page URL", form.action)
	}
}

func TestParseLoginForm_GETForm_Skipped(t *testing.T) {
	html := `
	<form action="/login" method="GET">
		<input type="text" name="user" />
		<input type="password" name="pass" />
	</form>`

	form := parseLoginForm(html, "https://example.com/login")
	if form != nil {
		t.Error("GET form with password should be skipped")
	}
}

func TestResolveFormAction(t *testing.T) {
	tests := []struct {
		action, page, want string
	}{
		{"/login", "https://example.com/auth", "https://example.com/login"},
		{"", "https://example.com/login", "https://example.com/login"},
		{"#", "https://example.com/login", "https://example.com/login"},
		{"https://auth.example.com/sso", "https://example.com/login", "https://auth.example.com/sso"},
		{"../api/login", "https://example.com/app/auth", "https://example.com/api/login"},
	}

	for _, tt := range tests {
		got := resolveFormAction(tt.action, tt.page)
		if got != tt.want {
			t.Errorf("resolveFormAction(%q, %q) = %q, want %q", tt.action, tt.page, got, tt.want)
		}
	}
}

func TestClassifyField_Patterns(t *testing.T) {
	tests := []struct {
		name      string
		inputType string
		wantRole  fieldRole
	}{
		{"password", "password", fieldRolePassword},
		{"email", "email", fieldRoleUsername},
		{"username", "text", fieldRoleUsername},
		{"user", "text", fieldRoleUsername},
		{"login", "text", fieldRoleUsername},
		{"correo", "text", fieldRoleUsername},
		{"csrf_token", "hidden", fieldRoleCSRF},
		{"_token", "hidden", fieldRoleCSRF},
		{"authenticity_token", "hidden", fieldRoleCSRF},
		{"redirect_url", "hidden", fieldRoleHidden},
		{"remember_me", "text", fieldRoleUnknown},
	}

	for _, tt := range tests {
		f := formField{name: tt.name, inputType: tt.inputType}
		got := classifyField(f)
		if got != tt.wantRole {
			t.Errorf("classifyField(%q, %q) = %d, want %d", tt.name, tt.inputType, got, tt.wantRole)
		}
	}
}
