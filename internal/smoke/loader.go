package smoke

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadScenarios walks dir recursively and parses all *.yaml files as Scenarios.
// Returns an error if any file fails validation.
func LoadScenarios(dir string) ([]Scenario, error) {
	var scenarios []Scenario
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		s, err := loadFile(path)
		if err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
		scenarios = append(scenarios, s)
		return nil
	})
	return scenarios, err
}

func loadFile(path string) (Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Scenario{}, fmt.Errorf("parse YAML: %w", err)
	}
	// Apply defaults.
	if s.Format == "" {
		s.Format = "table"
	}
	if s.TimeoutMs <= 0 {
		s.TimeoutMs = 10000
	}
	s.FilePath = path
	// Validate required fields.
	if s.Name == "" {
		return Scenario{}, fmt.Errorf("missing required field: name")
	}
	if s.Service == "" {
		return Scenario{}, fmt.Errorf("missing required field: service")
	}
	return s, nil
}
