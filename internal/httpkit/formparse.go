package httpkit

import (
	"net/url"
	"regexp"
	"strings"
)

// fieldRole classifies what role a form field plays in a login form.
type fieldRole int

const (
	fieldRoleUnknown  fieldRole = iota
	fieldRoleUsername           // text/email field for username
	fieldRolePassword           // password field
	fieldRoleCSRF              // hidden CSRF token
	fieldRoleHidden            // other hidden field to preserve
)

// formField represents a single <input> in a login form.
type formField struct {
	name      string
	inputType string // "text", "password", "hidden", "email", etc.
	value     string // pre-filled value (for hidden fields)
	role      fieldRole
}

// loginForm represents a parsed login form from HTML.
type loginForm struct {
	action string // resolved POST URL
	fields []formField
}

// Regex patterns for form parsing.
// We use regex instead of html.Parse to keep dependencies minimal and handle
// malformed HTML that real-world login pages often have.
var (
	// Match <form ...> ... </form> blocks (non-greedy, case-insensitive)
	reForm = regexp.MustCompile(`(?is)<form\b([^>]*)>(.*?)</form>`)

	// Extract action="..." from form tag
	reFormAction = regexp.MustCompile(`(?i)action=["']([^"']*)["']`)

	// Extract method="..." from form tag
	reFormMethod = regexp.MustCompile(`(?i)method=["']([^"']*)["']`)

	// Match <input ...> (self-closing or not)
	reInput = regexp.MustCompile(`(?is)<input\b([^>]*)>`)

	// Extract attributes from an input tag
	reInputType  = regexp.MustCompile(`(?i)type=["']([^"']*)["']`)
	reInputName  = regexp.MustCompile(`(?i)name=["']([^"']*)["']`)
	reInputValue = regexp.MustCompile(`(?i)value=["']([^"']*)["']`)

	// Match <button type="submit"> or <input type="submit">
	reSubmit = regexp.MustCompile(`(?i)type=["']submit["']`)
)

// Username field name patterns (case-insensitive matching)
var usernamePatterns = []string{
	"user", "username", "login", "email", "mail",
	"usuario", "correo", "compte", "account",
	"log", "uid", "ident", "name",
}

// CSRF field name patterns
var csrfFieldPatterns = []string{
	"csrf", "xsrf", "_token", "authenticity_token",
	"__requestverificationtoken", "antiforgery",
	"nonce", "csrfmiddlewaretoken",
}

// parseLoginForm finds the login form in HTML and extracts its structure.
// It returns nil if no form with a password field is found.
func parseLoginForm(html, pageURL string) *loginForm {
	forms := reForm.FindAllStringSubmatch(html, -1)
	if len(forms) == 0 {
		return nil
	}

	// Find the form that contains a password input — that's the login form
	for _, formMatch := range forms {
		formAttrs := formMatch[1]
		formBody := formMatch[2]

		inputs := reInput.FindAllStringSubmatch(formBody, -1)
		if len(inputs) == 0 {
			continue
		}

		var fields []formField
		hasPassword := false

		for _, inputMatch := range inputs {
			attrs := inputMatch[1]

			f := formField{}

			// Extract type
			if m := reInputType.FindStringSubmatch(attrs); m != nil {
				f.inputType = strings.ToLower(m[1])
			} else {
				f.inputType = "text" // default
			}

			// Extract name
			if m := reInputName.FindStringSubmatch(attrs); m != nil {
				f.name = m[1]
			}

			// Extract value
			if m := reInputValue.FindStringSubmatch(attrs); m != nil {
				f.value = m[1]
			}

			// Skip submit buttons and nameless fields
			if f.inputType == "submit" || f.inputType == "button" || f.name == "" {
				continue
			}

			// Classify role
			f.role = classifyField(f)
			if f.inputType == "password" {
				hasPassword = true
			}

			fields = append(fields, f)
		}

		if !hasPassword {
			continue // not a login form
		}

		// If no field was classified as username, pick the text/email field
		// closest (in DOM order) BEFORE the password field
		ensureUsernameField(fields)

		// Resolve form action URL
		action := pageURL
		if m := reFormAction.FindStringSubmatch(formAttrs); m != nil && m[1] != "" {
			action = resolveFormAction(m[1], pageURL)
		}

		// Check method (only POST makes sense for login)
		if m := reFormMethod.FindStringSubmatch(formAttrs); m != nil {
			if strings.ToUpper(m[1]) == "GET" {
				continue // GET forms aren't login forms
			}
		}

		return &loginForm{
			action: action,
			fields: fields,
		}
	}

	return nil
}

// classifyField determines what role a form field plays.
func classifyField(f formField) fieldRole {
	if f.inputType == "password" {
		return fieldRolePassword
	}

	lower := strings.ToLower(f.name)

	// Check CSRF patterns first (hidden fields with csrf-like names)
	if f.inputType == "hidden" {
		for _, pattern := range csrfFieldPatterns {
			if strings.Contains(lower, pattern) {
				return fieldRoleCSRF
			}
		}
		return fieldRoleHidden
	}

	// Text/email fields — check if it's a username field
	if f.inputType == "text" || f.inputType == "email" || f.inputType == "tel" {
		// Email type is almost always the username
		if f.inputType == "email" {
			return fieldRoleUsername
		}
		for _, pattern := range usernamePatterns {
			if strings.Contains(lower, pattern) {
				return fieldRoleUsername
			}
		}
	}

	return fieldRoleUnknown
}

// ensureUsernameField ensures at least one field is marked as username.
// If none was auto-detected, pick the last text/email field before the password.
func ensureUsernameField(fields []formField) {
	hasUsername := false
	for _, f := range fields {
		if f.role == fieldRoleUsername {
			hasUsername = true
			break
		}
	}
	if hasUsername {
		return
	}

	// Find the text/email field closest before (or at) the password field
	lastTextIdx := -1
	for i, f := range fields {
		if f.role == fieldRolePassword {
			break
		}
		if f.inputType == "text" || f.inputType == "email" || f.inputType == "tel" {
			lastTextIdx = i
		}
	}
	if lastTextIdx >= 0 {
		fields[lastTextIdx].role = fieldRoleUsername
		return
	}

	// Fallback: first text-like field anywhere
	for i, f := range fields {
		if f.inputType == "text" || f.inputType == "email" || f.inputType == "tel" {
			fields[i].role = fieldRoleUsername
			return
		}
	}
}

// resolveFormAction resolves a potentially relative form action against the page URL.
func resolveFormAction(action, pageURL string) string {
	if action == "" || action == "#" {
		return pageURL
	}

	// Already absolute
	if strings.HasPrefix(action, "http://") || strings.HasPrefix(action, "https://") {
		return action
	}

	base, err := url.Parse(pageURL)
	if err != nil {
		return pageURL
	}

	ref, err := url.Parse(action)
	if err != nil {
		return pageURL
	}

	return base.ResolveReference(ref).String()
}
