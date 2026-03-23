package api

import (
	"net/http"
	"os"
	"path/filepath"

	"reconx/internal/config"
	"reconx/internal/tools"
)

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	redacted := s.Config.Redacted()
	writeJSON(w, 200, redacted)
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var newCfg config.Config
	if err := decodeJSON(r, &newCfg); err != nil {
		writeError(w, 400, "invalid JSON config")
		return
	}

	// Preserve masked API keys — don't overwrite real values with "***"
	if newCfg.APIKeys.Shodan == "***" {
		newCfg.APIKeys.Shodan = s.Config.APIKeys.Shodan
	}
	if newCfg.APIKeys.CensysID == "***" {
		newCfg.APIKeys.CensysID = s.Config.APIKeys.CensysID
	}
	if newCfg.APIKeys.CensysSecret == "***" {
		newCfg.APIKeys.CensysSecret = s.Config.APIKeys.CensysSecret
	}
	if newCfg.APIKeys.GithubToken == "***" {
		newCfg.APIKeys.GithubToken = s.Config.APIKeys.GithubToken
	}

	// Save to global config file
	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, 500, "cannot determine home directory")
		return
	}

	configPath := filepath.Join(home, ".reconx", "config.yaml")
	if err := config.Save(&newCfg, configPath); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	// Reload config in memory
	loaded, err := config.Load()
	if err != nil {
		writeError(w, 500, "config saved but reload failed: "+err.Error())
		return
	}

	*s.Config = *loaded
	redacted := s.Config.Redacted()
	writeJSON(w, 200, redacted)
}

func (s *Server) checkTools(w http.ResponseWriter, r *http.Request) {
	allTools := tools.AllToolNames()
	results := tools.CheckAll(allTools)
	writeJSON(w, 200, results)
}
