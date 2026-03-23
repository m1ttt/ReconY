package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Workflow defines a recon execution plan.
type Workflow struct {
	Name        string        `yaml:"name" json:"name"`
	Description string        `yaml:"description" json:"description"`
	Phases      []PhaseConfig `yaml:"phases" json:"phases"`
}

// PhaseConfig configures a single phase within a workflow.
type PhaseConfig struct {
	ID      int          `yaml:"id" json:"id"`
	Enabled bool         `yaml:"enabled" json:"enabled"`
	Tools   []ToolConfig `yaml:"tools" json:"tools"`
}

// ToolConfig configures a single tool within a phase.
type ToolConfig struct {
	Name      string            `yaml:"name" json:"name"`
	Enabled   bool              `yaml:"enabled" json:"enabled"`
	Wordlists map[string]string `yaml:"wordlists,omitempty" json:"wordlists,omitempty"`
	Options   map[string]any    `yaml:"options,omitempty" json:"options,omitempty"`
	Timeout   string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Threads   int               `yaml:"threads,omitempty" json:"threads,omitempty"`
}

// Parse parses a YAML workflow definition.
func Parse(data []byte) (*Workflow, error) {
	var w Workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parsing workflow: %w", err)
	}
	return &w, nil
}

// Marshal serializes a workflow to YAML.
func (w *Workflow) Marshal() ([]byte, error) {
	return yaml.Marshal(w)
}

// EnabledPhases returns only the phases that are enabled.
func (w *Workflow) EnabledPhases() []PhaseConfig {
	var result []PhaseConfig
	for _, p := range w.Phases {
		if p.Enabled {
			result = append(result, p)
		}
	}
	return result
}

// EnabledTools returns only the enabled tools for a given phase.
func (p *PhaseConfig) EnabledTools() []ToolConfig {
	var result []ToolConfig
	for _, t := range p.Tools {
		if t.Enabled {
			result = append(result, t)
		}
	}
	return result
}

// PhaseIDs returns the list of enabled phase IDs.
func (w *Workflow) PhaseIDs() []int {
	var ids []int
	for _, p := range w.Phases {
		if p.Enabled {
			ids = append(ids, p.ID)
		}
	}
	return ids
}
