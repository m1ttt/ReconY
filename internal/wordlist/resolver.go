package wordlist

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolver resolves logical wordlist names to absolute file paths.
type Resolver struct {
	seclistsPath string
}

// NewResolver creates a resolver with the given SecLists base path.
func NewResolver(seclistsPath string) *Resolver {
	return &Resolver{seclistsPath: seclistsPath}
}

// Resolve converts a relative wordlist path to an absolute path under SecLists.
// If the input is already an absolute path, it's returned as-is.
func (r *Resolver) Resolve(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		if _, err := os.Stat(relPath); err != nil {
			return "", fmt.Errorf("wordlist not found: %s", relPath)
		}
		return relPath, nil
	}

	abs := filepath.Join(r.seclistsPath, relPath)
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("wordlist not found: %s (resolved to %s)", relPath, abs)
	}
	return abs, nil
}

// Exists checks if a wordlist file exists.
func (r *Resolver) Exists(relPath string) bool {
	_, err := r.Resolve(relPath)
	return err == nil
}

// SecListsAvailable checks if the SecLists directory exists and is populated.
func (r *Resolver) SecListsAvailable() bool {
	info, err := os.Stat(r.seclistsPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
