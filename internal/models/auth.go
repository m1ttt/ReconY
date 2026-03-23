package models

// AuthType defines the authentication method.
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBasic  AuthType = "basic"  // HTTP Basic Auth
	AuthTypeForm   AuthType = "form"   // POST to login URL
	AuthTypeCookie AuthType = "cookie" // raw cookie string
	AuthTypeBearer AuthType = "bearer" // Authorization: Bearer
	AuthTypeHeader AuthType = "header" // custom header
)

// AuthCredential stores authentication credentials for a workspace.
type AuthCredential struct {
	ID          string   `json:"id" db:"id"`
	WorkspaceID string   `json:"workspace_id" db:"workspace_id"`
	Name        string   `json:"name" db:"name"`
	AuthType    AuthType `json:"auth_type" db:"auth_type"`
	Username    *string  `json:"username,omitempty" db:"username"`
	Password    *string  `json:"password,omitempty" db:"password"`
	LoginURL    *string  `json:"login_url,omitempty" db:"login_url"`
	LoginBody   *string  `json:"login_body,omitempty" db:"login_body"` // template: {"user":"{{username}}","pass":"{{password}}"}
	Token       *string  `json:"token,omitempty" db:"token"`
	HeaderName  *string  `json:"header_name,omitempty" db:"header_name"`
	HeaderValue *string  `json:"header_value,omitempty" db:"header_value"`
	IsActive    bool     `json:"is_active" db:"is_active"`
	CreatedAt   string   `json:"created_at" db:"created_at"`
	UpdatedAt   string   `json:"updated_at" db:"updated_at"`
}
