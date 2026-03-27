package config

// Config is the main configuration struct for ReconX.
// Three levels: defaults (embedded) → global (~/.reconx/config.yaml) → per-workspace.
type Config struct {
	General   GeneralConfig         `yaml:"general" json:"general"`
	APIKeys   APIKeysConfig         `yaml:"api_keys" json:"api_keys"`
	Proxy     ProxyConfig           `yaml:"proxy" json:"proxy"`
	Tools     map[string]ToolConfig `yaml:"tools" json:"tools"`
	Wordlists WordlistsConfig      `yaml:"wordlists" json:"wordlists"`
}

// GeneralConfig holds top-level settings.
type GeneralConfig struct {
	DBPath             string `yaml:"db_path" json:"db_path"`
	ScreenshotsDir     string `yaml:"screenshots_dir" json:"screenshots_dir"`
	SecListsPath       string `yaml:"seclists_path" json:"seclists_path"`
	MaxConcurrentTools int    `yaml:"max_concurrent_tools" json:"max_concurrent_tools"`
	DefaultWorkflow    string `yaml:"default_workflow" json:"default_workflow"`
	APIListenAddr      string `yaml:"api_listen_addr" json:"api_listen_addr"`
	RateLimitPerHost   int    `yaml:"rate_limit_per_host" json:"rate_limit_per_host"`
	MaxRetries         int    `yaml:"max_retries" json:"max_retries"`
}

// ProxyConfig holds proxy and VPN rotation settings.
type ProxyConfig struct {
	URL              string   `yaml:"url" json:"url"`
	RotationEnabled  bool     `yaml:"rotation_enabled" json:"rotation_enabled"`
	RotateEveryN     int      `yaml:"rotate_every_n" json:"rotate_every_n"`
	RotateInterval   string   `yaml:"rotate_interval" json:"rotate_interval"`
	MullvadCLI       bool     `yaml:"mullvad_cli" json:"mullvad_cli"`
	MullvadLocations []string `yaml:"mullvad_locations" json:"mullvad_locations"`
}

// APIKeysConfig holds API keys for optional integrations.
type APIKeysConfig struct {
	Shodan        string `yaml:"shodan" json:"shodan"`
	CensysID      string `yaml:"censys_id" json:"censys_id"`
	CensysSecret  string `yaml:"censys_secret" json:"censys_secret"`
	GithubToken   string `yaml:"github_token" json:"github_token"`
	OpenAIKey     string `yaml:"openai_key" json:"openai_key"`
	OpenAIBaseURL string `yaml:"openai_base_url" json:"openai_base_url"`
	OpenAIModel   string `yaml:"openai_model" json:"openai_model"`
	TavilyKey     string `yaml:"tavily_key" json:"tavily_key"`
	AIServiceURL  string `yaml:"ai_service_url" json:"ai_service_url"`
}

// ToolConfig is the per-tool configuration. Each tool can override these.
type ToolConfig struct {
	Enabled         *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Threads         *int              `yaml:"threads,omitempty" json:"threads,omitempty"`
	Timeout         string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RateLimit       *int              `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	ExtraArgs       []string          `yaml:"extra_args,omitempty" json:"extra_args,omitempty"`
	Options         map[string]any    `yaml:"options,omitempty" json:"options,omitempty"`
	WordlistOverrides map[string]string `yaml:"wordlist_overrides,omitempty" json:"wordlist_overrides,omitempty"`
}

// WordlistsConfig maps logical names to paths relative to SecListsPath.
type WordlistsConfig struct {
	// DNS
	DNSQuick      string `yaml:"dns_quick" json:"dns_quick"`
	DNSStandard   string `yaml:"dns_standard" json:"dns_standard"`
	DNSAggressive string `yaml:"dns_aggressive" json:"dns_aggressive"`

	// Web content
	WebQuick      string `yaml:"web_quick" json:"web_quick"`
	WebStandard   string `yaml:"web_standard" json:"web_standard"`
	WebAggressive string `yaml:"web_aggressive" json:"web_aggressive"`

	// API
	APIEndpoints string `yaml:"api_endpoints" json:"api_endpoints"`
	APIWild      string `yaml:"api_wild" json:"api_wild"`

	// Parameters
	Params string `yaml:"params" json:"params"`

	// CMS-specific
	CMSWordpress string `yaml:"cms_wordpress" json:"cms_wordpress"`
	CMSDrupal    string `yaml:"cms_drupal" json:"cms_drupal"`
	CMSJoomla    string `yaml:"cms_joomla" json:"cms_joomla"`

	// Tech-specific
	TechPHP  string `yaml:"tech_php" json:"tech_php"`
	TechJava string `yaml:"tech_java" json:"tech_java"`
	TechRoR  string `yaml:"tech_ror" json:"tech_ror"`

	// Fuzzing
	LFI  string `yaml:"lfi" json:"lfi"`
	XSS  string `yaml:"xss" json:"xss"`
	SQLi string `yaml:"sqli" json:"sqli"`
	SSRF string `yaml:"ssrf" json:"ssrf"`
}

// IsToolEnabled returns whether a tool is enabled, defaulting to true if not set.
func (c *Config) IsToolEnabled(name string) bool {
	tc, ok := c.Tools[name]
	if !ok {
		return true
	}
	if tc.Enabled == nil {
		return true
	}
	return *tc.Enabled
}

// GetToolConfig returns the config for a tool, or empty config if not found.
func (c *Config) GetToolConfig(name string) ToolConfig {
	if tc, ok := c.Tools[name]; ok {
		return tc
	}
	return ToolConfig{}
}

// Redacted returns a copy of the config with API keys masked.
func (c *Config) Redacted() Config {
	copy := *c
	if copy.APIKeys.Shodan != "" {
		copy.APIKeys.Shodan = "***"
	}
	if copy.APIKeys.CensysID != "" {
		copy.APIKeys.CensysID = "***"
	}
	if copy.APIKeys.CensysSecret != "" {
		copy.APIKeys.CensysSecret = "***"
	}
	if copy.APIKeys.GithubToken != "" {
		copy.APIKeys.GithubToken = "***"
	}
	if copy.APIKeys.OpenAIKey != "" {
		copy.APIKeys.OpenAIKey = "***"
	}
	if copy.APIKeys.TavilyKey != "" {
		copy.APIKeys.TavilyKey = "***"
	}
	return copy
}
