package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// ToolInfo contains metadata about an external tool.
type ToolInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CheckAll verifies which tools are available and returns their info.
func CheckAll(names []string) []ToolInfo {
	var results []ToolInfo
	for _, name := range names {
		info := ToolInfo{Name: name}

		path, err := exec.LookPath(name)
		if err != nil {
			info.Error = fmt.Sprintf("not found in PATH")
			results = append(results, info)
			continue
		}

		info.Available = true
		info.Path = path

		// Try to get version
		version, err := getVersion(name)
		if err == nil {
			info.Version = version
		}

		results = append(results, info)
	}
	return results
}

// getVersion tries common version flags to get tool version.
func getVersion(name string) (string, error) {
	for _, flag := range []string{"--version", "-version", "version", "-v"} {
		out, err := exec.Command(name, flag).Output()
		if err == nil {
			version := strings.TrimSpace(string(out))
			// Take first line only
			if idx := strings.Index(version, "\n"); idx > 0 {
				version = version[:idx]
			}
			if version != "" {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("could not determine version")
}

// AllToolNames returns the names of all external tools used by ReconX.
func AllToolNames() []string {
	return []string{
		"subfinder",
		"amass",
		"puredns",
		"nmap",
		"httpx",
		"katana",
		"ffuf",
		"nuclei",
		"gowitness",
		"waybackurls",
		"gau",
		"jsluice",
		"paramspider",
		"cmseek",
	}
}
