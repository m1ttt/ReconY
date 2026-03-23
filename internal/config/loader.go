package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Load reads and merges configuration from all levels:
// 1. Defaults (hardcoded)
// 2. Global file (~/.reconx/config.yaml)
// 3. Workspace overrides (JSON from DB)
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load global config file
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".reconx", "config.yaml")
		if err := mergeFromFile(cfg, globalPath); err != nil {
			return nil, fmt.Errorf("loading global config: %w", err)
		}
	}

	// Override from environment variables
	applyEnvOverrides(cfg)

	return cfg, nil
}

// LoadWithWorkspace loads the base config and merges workspace-specific overrides.
func LoadWithWorkspace(workspaceConfigJSON string) (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	if workspaceConfigJSON != "" && workspaceConfigJSON != "{}" {
		if err := mergeFromJSON(cfg, workspaceConfigJSON); err != nil {
			return nil, fmt.Errorf("merging workspace config: %w", err)
		}
	}

	return cfg, nil
}

// mergeFromFile reads a YAML file and applies it to the existing config.
// For the global config file, we unmarshal directly on top of defaults
// so that explicit false/zero values are preserved.
func mergeFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, skip
		}
		return err
	}

	// Unmarshal directly onto the config so explicit false/zero values work
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	return nil
}

// mergeFromJSON merges a JSON config string into the existing config.
func mergeFromJSON(cfg *Config, jsonStr string) error {
	var overlay Config
	if err := json.Unmarshal([]byte(jsonStr), &overlay); err != nil {
		return fmt.Errorf("parsing JSON config: %w", err)
	}
	mergeConfig(cfg, &overlay)
	return nil
}

// mergeConfig merges non-zero values from overlay into base.
func mergeConfig(base, overlay *Config) {
	// General
	mergeString(&base.General.DBPath, overlay.General.DBPath)
	mergeString(&base.General.ScreenshotsDir, overlay.General.ScreenshotsDir)
	mergeString(&base.General.SecListsPath, overlay.General.SecListsPath)
	mergeString(&base.General.DefaultWorkflow, overlay.General.DefaultWorkflow)
	mergeString(&base.General.APIListenAddr, overlay.General.APIListenAddr)
	if overlay.General.MaxConcurrentTools > 0 {
		base.General.MaxConcurrentTools = overlay.General.MaxConcurrentTools
	}

	if overlay.General.RateLimitPerHost > 0 {
		base.General.RateLimitPerHost = overlay.General.RateLimitPerHost
	}
	if overlay.General.MaxRetries > 0 {
		base.General.MaxRetries = overlay.General.MaxRetries
	}

	// Proxy
	mergeString(&base.Proxy.URL, overlay.Proxy.URL)
	mergeString(&base.Proxy.RotateInterval, overlay.Proxy.RotateInterval)
	if overlay.Proxy.RotationEnabled {
		base.Proxy.RotationEnabled = true
	}
	if overlay.Proxy.MullvadCLI {
		base.Proxy.MullvadCLI = true
	}
	if overlay.Proxy.RotateEveryN > 0 {
		base.Proxy.RotateEveryN = overlay.Proxy.RotateEveryN
	}
	if len(overlay.Proxy.MullvadLocations) > 0 {
		base.Proxy.MullvadLocations = overlay.Proxy.MullvadLocations
	}

	// API Keys
	mergeString(&base.APIKeys.Shodan, overlay.APIKeys.Shodan)
	mergeString(&base.APIKeys.CensysID, overlay.APIKeys.CensysID)
	mergeString(&base.APIKeys.CensysSecret, overlay.APIKeys.CensysSecret)
	mergeString(&base.APIKeys.GithubToken, overlay.APIKeys.GithubToken)

	// Tools: merge each tool config
	if overlay.Tools != nil {
		if base.Tools == nil {
			base.Tools = make(map[string]ToolConfig)
		}
		for name, overlayTool := range overlay.Tools {
			baseTool, exists := base.Tools[name]
			if !exists {
				base.Tools[name] = overlayTool
				continue
			}
			mergeToolConfig(&baseTool, &overlayTool)
			base.Tools[name] = baseTool
		}
	}

	// Wordlists: merge non-empty fields via reflection
	mergeWordlists(&base.Wordlists, &overlay.Wordlists)
}

func mergeToolConfig(base, overlay *ToolConfig) {
	if overlay.Enabled != nil {
		base.Enabled = overlay.Enabled
	}
	if overlay.Threads != nil {
		base.Threads = overlay.Threads
	}
	if overlay.Timeout != "" {
		base.Timeout = overlay.Timeout
	}
	if overlay.RateLimit != nil {
		base.RateLimit = overlay.RateLimit
	}
	if len(overlay.ExtraArgs) > 0 {
		base.ExtraArgs = overlay.ExtraArgs
	}
	if overlay.Options != nil {
		if base.Options == nil {
			base.Options = make(map[string]any)
		}
		for k, v := range overlay.Options {
			base.Options[k] = v
		}
	}
	if overlay.WordlistOverrides != nil {
		if base.WordlistOverrides == nil {
			base.WordlistOverrides = make(map[string]string)
		}
		for k, v := range overlay.WordlistOverrides {
			base.WordlistOverrides[k] = v
		}
	}
}

func mergeWordlists(base, overlay *WordlistsConfig) {
	bv := reflect.ValueOf(base).Elem()
	ov := reflect.ValueOf(overlay).Elem()
	for i := 0; i < bv.NumField(); i++ {
		if s := ov.Field(i).String(); s != "" {
			bv.Field(i).SetString(s)
		}
	}
}

func mergeString(base *string, overlay string) {
	if overlay != "" {
		*base = overlay
	}
}

// applyEnvOverrides reads environment variables to override config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("RECONX_DB"); v != "" {
		cfg.General.DBPath = v
	}
	if v := os.Getenv("RECONX_SECLISTS"); v != "" {
		cfg.General.SecListsPath = v
	}
	if v := os.Getenv("RECONX_LISTEN"); v != "" {
		cfg.General.APIListenAddr = v
	}
	if v := os.Getenv("RECONX_SHODAN_KEY"); v != "" {
		cfg.APIKeys.Shodan = v
	}
	if v := os.Getenv("RECONX_CENSYS_ID"); v != "" {
		cfg.APIKeys.CensysID = v
	}
	if v := os.Getenv("RECONX_CENSYS_SECRET"); v != "" {
		cfg.APIKeys.CensysSecret = v
	}
	if v := os.Getenv("RECONX_GITHUB_TOKEN"); v != "" {
		cfg.APIKeys.GithubToken = v
	}
}

// Save writes the config to a YAML file.
func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
