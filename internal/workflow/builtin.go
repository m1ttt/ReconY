package workflow

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed builtins/*.yaml
var builtinsFS embed.FS

// LoadBuiltins returns all built-in workflows embedded in the binary.
func LoadBuiltins() (map[string]*Workflow, error) {
	entries, err := builtinsFS.ReadDir("builtins")
	if err != nil {
		return nil, fmt.Errorf("reading builtins: %w", err)
	}

	workflows := make(map[string]*Workflow)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := builtinsFS.ReadFile("builtins/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		w, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}

		workflows[w.Name] = w
	}

	return workflows, nil
}

// LoadCustom loads custom workflows from a directory (e.g., ~/.reconx/workflows/).
func LoadCustom(dir string) (map[string]*Workflow, error) {
	workflows := make(map[string]*Workflow)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return workflows, nil
		}
		return nil, fmt.Errorf("reading custom workflows: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		w, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}

		workflows[w.Name] = w
	}

	return workflows, nil
}

// Registry holds all available workflows (builtin + custom).
type Registry struct {
	workflows map[string]*Workflow
	builtin   map[string]bool
}

// NewRegistry creates a workflow registry loading builtins and optionally custom workflows.
func NewRegistry(customDir string) (*Registry, error) {
	builtins, err := LoadBuiltins()
	if err != nil {
		return nil, err
	}

	r := &Registry{
		workflows: make(map[string]*Workflow),
		builtin:   make(map[string]bool),
	}

	for name, w := range builtins {
		r.workflows[name] = w
		r.builtin[name] = true
	}

	if customDir != "" {
		custom, err := LoadCustom(customDir)
		if err != nil {
			return nil, err
		}
		for name, w := range custom {
			r.workflows[name] = w
		}
	}

	return r, nil
}

// Get returns a workflow by name.
func (r *Registry) Get(name string) (*Workflow, bool) {
	w, ok := r.workflows[name]
	return w, ok
}

// List returns all workflow names sorted alphabetically.
func (r *Registry) List() []WorkflowInfo {
	var result []WorkflowInfo
	for name, w := range r.workflows {
		result = append(result, WorkflowInfo{
			Name:        name,
			Description: w.Description,
			IsBuiltin:   r.builtin[name],
			PhaseIDs:    w.PhaseIDs(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// WorkflowInfo is a summary for listing workflows.
type WorkflowInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
	PhaseIDs    []int  `json:"phase_ids"`
}
